package umpire

import (
	"bufio"
	"encoding/json"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	"golang.org/x/net/context"
	"io"
	"log"
	"net"
	"os"
	_ "strings"
	"time"
)

var configMap = map[string]struct {
	Cmd   []string
	Image string
}{
	"cpp": {[]string{"-stream=true"}, "phluent/clang"},
}

type Problem struct {
	Id string `json:"id"`
}

type InMemoryFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Payload struct {
	Language string          `json:"language"`
	Files    []*InMemoryFile `json:"files"`
	Problem  *Problem        `json:"problem"`
	Stdin    string          `json:"stdin"`
}

func writeConn(conn net.Conn, data []byte) error {
	log.Printf("Want to write %d bytes", len(data))
	var start, c int
	var err error
	for {
		if c, err = conn.Write(data[start:]); err != nil {
			return err
		}
		start += c
		log.Printf("Wrote %d of %d bytes", start, len(data))
		if c == 0 || start == len(data) {
			break
		}
	}
	return nil
}

type DockerEvalResult struct {
	containerId string
	Done        chan struct{}
	Stdout      io.ReadCloser
	Stderr      io.ReadCloser
	Cancel      context.CancelFunc
	Cleanup     func() error
}

func DockerRun(cli *client.Client, payload *Payload, w io.Writer) error {
	dockerEvalResult, err := dockerEval(context.Background(), cli, payload)
	if err != nil {
		return err
	}
	readWrite := func(readFrom io.ReadCloser, writeTo io.Writer) {
		r, w := io.Pipe()
		mw := io.MultiWriter(writeTo, w)
		go func() {
			defer readFrom.Close()
			defer r.Close()
			io.Copy(mw, readFrom)
		}()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			log.Printf("text: %q", text)
		}
	}
	go readWrite(dockerEvalResult.Stdout, os.Stdout)
	go readWrite(dockerEvalResult.Stderr, os.Stderr)

	// go func() {
	// 	io.Copy(os.Stdout, dockerEvalResult.Stdout)
	// }()
	// go func() {
	// 	io.Copy(os.Stdout, dockerEvalResult.StdoutSpl)
	// }()
	defer dockerEvalResult.Cleanup()
	<-dockerEvalResult.Done
	return nil
}

func dockerEval(ctx context.Context, cli *client.Client, payload *Payload) (*DockerEvalResult, error) {
	cfg := configMap[payload.Language]
	config := &container.Config{
		Image:       cfg.Image,
		Cmd:         cfg.Cmd,
		AttachStdin: true,
		OpenStdin:   true,
		StdinOnce:   false,
	}

	resp, err := cli.ContainerCreate(ctx, config, &container.HostConfig{}, &network.NetworkingConfig{}, "")
	if err != nil {
		return nil, err
	}
	containerId := resp.ID

	err = cli.ContainerStart(ctx, containerId, types.ContainerStartOptions{})
	if err != nil {
		return nil, err
	}

	stdout, err := cli.ContainerLogs(ctx, containerId, types.ContainerLogsOptions{
		ShowStdout: true,
		Follow:     true,
	})
	if err != nil {
		return nil, err
	}
	stderr, err := cli.ContainerLogs(ctx, containerId, types.ContainerLogsOptions{
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	hijackedResp, err := cli.ContainerAttach(ctx, containerId, types.ContainerAttachOptions{
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return nil, err
	}
	go func(data []byte, conn net.Conn) {
		defer func() { log.Println("Done writing") }()
		defer conn.Close()
		err := writeConn(conn, data)
		if err != nil {
			log.Printf("Error while writing to connection: %v", err)
		}
	}(data, hijackedResp.Conn)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan struct{})
	go func() {
		_, err = cli.ContainerWait(ctx, containerId)
		if err != nil {
			log.Printf("here: %v", err)
			done <- struct{}{}
			return
		}
		done <- struct{}{}
	}()

	rStdout, wStdout := io.Pipe()
	rStderr, wStderr := io.Pipe()

	scanLines := func(r io.Reader, w io.Writer) {
		defer wStdout.Close()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			text = text[8:]
			if len(text) > 0 {
				if err := SpecialWrite(w, text); err != nil {
					log.Fatalf("system err: %v", err)
					return
				}
			}
			if err := scanner.Err(); err != nil {
				log.Fatalf("scanner err: %v", err)
				return
			}
		}
	}

	go scanLines(stdout, wStdout)
	go scanLines(stderr, wStderr)

	cleanup := func() error {
		return cli.ContainerRemove(context.Background(), containerId, types.ContainerRemoveOptions{Force: true})
	}
	return &DockerEvalResult{containerId, done, rStdout, rStderr, cancel, cleanup}, nil
}
