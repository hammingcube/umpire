package main

import (
	"fmt"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"net"
	"time"
)

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
	done        chan struct{}
	reader      io.Reader
	cancel      context.CancelFunc
	cleanup     func() error
}

func DockerEval(cli *client.Client, srcDir string, language string, testcase io.Reader) (*DockerEvalResult, error) {
	configMap := map[string]struct {
		Cmd   []string
		Image string
	}{
		"cpp": {[]string{"sh", "-c", "g++ -std=c++11 *.cpp -o binary.exe && ./binary.exe"}, "gcc"},
	}

	cfg := configMap["cpp"]

	config := &container.Config{
		Cmd:         cfg.Cmd,
		Image:       cfg.Image,
		WorkingDir:  "/app",
		AttachStdin: true,
		OpenStdin:   true,
		StdinOnce:   true,
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/app", srcDir),
		},
	}
	resp, err := cli.ContainerCreate(context.Background(), config, hostConfig, &network.NetworkingConfig{}, "")
	if err != nil {
		return nil, err
	}
	containerId := resp.ID
	err = cli.ContainerStart(context.Background(), containerId, types.ContainerStartOptions{})
	if err != nil {
		return nil, err
	}
	reader, err := cli.ContainerLogs(context.Background(), containerId, types.ContainerLogsOptions{
		ShowStdout: true,
		Follow:     true,
	})
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(testcase)
	if err != nil {
		return nil, err
	}
	time.Sleep(1 * time.Second)
	hijackedResp, err := cli.ContainerAttach(context.Background(), containerId, types.ContainerAttachOptions{
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
			panic(err)
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
	cleanup := func() error {
		return cli.ContainerRemove(context.Background(), containerId, types.ContainerRemoveOptions{Force: true})
	}
	return &DockerEvalResult{containerId, done, reader, cancel, cleanup}, nil
}
