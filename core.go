package umpire

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/labstack/gommon/log"
	"io/ioutil"
	"net"
)

type PayloadResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"stderr"`
}

func payloadRun(ctx context.Context, cli *client.Client, payload *Payload) (*PayloadResult, error) {
	var configMap = map[string]struct {
		Cmd   []string
		Image string
	}{
		"cpp":        {[]string{"-stream=false"}, "phluent/clang"},
		"python":     {[]string{"-stream=false"}, "phluent/python"},
		"javascript": {[]string{"-stream=false"}, "phluent/javascript"},
		"typescript": {[]string{"-stream=false"}, "phluent/typescript"},
	}
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

	defer func() {
		log.Infof("Cleaning up docker container %s", containerId)
		cli.ContainerRemove(context.Background(), containerId, types.ContainerRemoveOptions{Force: true})
	}()

	err = cli.ContainerStart(ctx, containerId, types.ContainerStartOptions{})
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
		defer func() { log.Printf("done writing to attached stdin") }()
		defer conn.Close()
		err := writeConn(conn, data)
		if err != nil {
			log.Printf("Error while writing to connection: %v", err)
		}
	}(data, hijackedResp.Conn)

	cli.ContainerWait(ctx, containerId)
	stdout, err := cli.ContainerLogs(ctx, containerId, types.ContainerLogsOptions{
		ShowStdout: true,
		Follow:     true,
	})
	if err != nil {
		return nil, err
	}
	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}
	v := &PayloadResult{}
	if err := json.NewDecoder(bytes.NewReader(out[8:])).Decode(v); err != nil {
		return nil, err
	}
	return v, nil
}
