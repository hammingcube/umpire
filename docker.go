package umpire

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/labstack/gommon/log"
	"io"
	"net"
	_ "os"
	_ "strings"
	"sync"
	"time"
)

var configMap = map[string]struct {
	Cmd   []string
	Image string
}{
	"cpp":        {[]string{"-stream=true"}, "phluent/clang"},
	"python":     {[]string{"-stream=true"}, "phluent/python"},
	"javascript": {[]string{"-stream=true"}, "phluent/javascript"},
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

func SpecialWrite(w io.Writer, text string) error {
	n, err := w.Write([]byte(text + "\n"))
	if n != len(text)+1 || (err != nil && err != io.EOF) {
		errorMsg := fmt.Sprintf("Error while writing %d bytes, wrote only %d bytes. Err: %v", len(text)+1, n, err)
		return errors.New(errorMsg)
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

func DockerRun(ctx context.Context, cli *client.Client, payload *Payload, wStdout io.Writer, wStderr io.Writer) error {
	dockerEvalResult, err := dockerEval(ctx, cli, payload)
	if err != nil {
		return err
	}
	defer dockerEvalResult.Cleanup()
	var wg sync.WaitGroup
	readWrite := func(readFrom io.ReadCloser, writeTo io.Writer, wg *sync.WaitGroup) {
		r, w := io.Pipe()
		mw := io.MultiWriter(writeTo, w)
		go func(wg *sync.WaitGroup) {
			defer func() { wg.Done() }()
			defer readFrom.Close()
			defer r.Close()
			io.Copy(mw, readFrom)
		}(wg)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			log.Printf("text: %q", text)
		}
	}
	wg.Add(2)
	go readWrite(dockerEvalResult.Stdout, wStdout, &wg)
	go readWrite(dockerEvalResult.Stderr, wStderr, &wg)

	select {
	case <-dockerEvalResult.Done:
		wg.Wait()
		log.Printf("Finished both read-write jobs again")
		return nil
	case <-ctx.Done():
		log.Printf("Context cancelled")
		dockerEvalResult.Cleanup()
		return errors.New("Context cancelled")
	}
}

func DockerJudge(ctx context.Context, cli *client.Client, payload *Payload, wStdout io.Writer, wStderr io.Writer, expected *bufio.Scanner) error {
	dockerEvalResult, err := dockerEval(ctx, cli, payload)
	if err != nil {
		return err
	}
	defer dockerEvalResult.Cleanup()
	errChan := make(chan error)
	var wg sync.WaitGroup
	readWrite := func(readFrom io.ReadCloser, writeTo io.Writer, source string, wg *sync.WaitGroup) {
		defer wg.Done()
		r, w := io.Pipe()
		scanner := bufio.NewScanner(r)
		go func(w io.WriteCloser) {
			defer readFrom.Close()
			defer w.Close()
			multiWriteTo := io.MultiWriter(writeTo, w)
			io.Copy(multiWriteTo, readFrom)
		}(w)
		fullText := ""
		for scanner.Scan() {
			text := scanner.Text()
			fullText += text
			log.Infof("scanning %s: %s", source, text)
			if source == "stdout" {
				expected.Scan()
				text1, text2 := text, expected.Text()
				log.Printf("got text: %q", text1)
				log.Printf("want text: %q", text2)
				if text1 != text2 {
					log.Printf("got stdout error: %s %s", text1, text2)
					errChan <- errors.New(fmt.Sprintf("Mismatch Error: got %s, expected %s", text1, text2))
					return
				}
			}
		}
		if source == "stderr" && fullText != "" {
			errChan <- fmt.Errorf("stderr error: %s", fullText)
			return
		}
		errChan <- nil
		return
	}
	wg.Add(2)
	go readWrite(dockerEvalResult.Stdout, wStdout, "stdout", &wg)
	go readWrite(dockerEvalResult.Stderr, wStderr, "stderr", &wg)

	go func() {
		log.Info("Waiting for read-write jobs to finish")
		wg.Wait()
		log.Info("Finished read-write jobs")
	}()
	var chanErr error
	select {
	case <-dockerEvalResult.Done:
		log.Printf("Finished both read-write jobs again")
		chanErr = <-errChan
	case <-ctx.Done():
		log.Printf("Original context cancelled: calling docker cleanup")
		dockerEvalResult.Cleanup()
		chanErr = <-errChan
	case chanErr = <-errChan:
	}
	log.Infof("Got one of the channel errors: %v", chanErr)
	if chanErr != nil {
		go func() {
			err := <-errChan
			log.Infof("Btw, the other channel error: %v", err)
		}()
		return chanErr
	}
	return <-errChan
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
		defer func() { log.Printf("dockerEval: Done writing") }()
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

	scanLines := func(r io.Reader, w io.WriteCloser) {
		defer w.Close()
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
		log.Info("Docker cleanup called")
		return cli.ContainerRemove(context.Background(), containerId, types.ContainerRemoveOptions{Force: true})
	}
	return &DockerEvalResult{containerId, done, rStdout, rStderr, cancel, cleanup}, nil
}
