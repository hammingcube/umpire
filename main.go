package main

import (
	"fmt"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	"golang.org/x/net/context"
	"io"
	"log"
	"os"
	"time"
)

func main() {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	options := types.ImageListOptions{All: true}
	images, err := cli.ImageList(context.Background(), options)
	if err != nil {
		panic(err)
	}

	for _, c := range images {
		fmt.Println(c.RepoTags)
	}

	config := &container.Config{
		Cmd:         []string{"sh", "-c", "g++ -std=c++11 main.cpp -o binary.exe && ./binary.exe"},
		Image:       "gcc",
		WorkingDir:  "/app",
		AttachStdin: true,
		OpenStdin:   true,
		StdinOnce:   true,
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			"/Users/madhavjha/src/github.com/maddyonline/moredocker:/app",
		},
	}
	resp, err := cli.ContainerCreate(context.Background(), config, hostConfig, &network.NetworkingConfig{}, "myapp")
	if err != nil {
		panic(err)
	}
	containerId := resp.ID

	err = cli.ContainerStart(context.Background(), containerId, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println("OK")

	reader, err := cli.ContainerLogs(context.Background(), containerId, types.ContainerLogsOptions{
		ShowStdout: true,
		Follow:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		_, err = io.Copy(os.Stdout, reader)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
	}()

	time.Sleep(5 * time.Second)

	hijackedResp, err := cli.ContainerAttach(context.Background(), containerId, types.ContainerAttachOptions{
		Stdin:  true,
		Stream: true,
	})

	if err != nil {
		panic(err)
	}
	go func() {
		defer func() { fmt.Println("Done writing") }()
		defer hijackedResp.Conn.Close()
		data := []byte("bye\ncool\n")
		n, err := hijackedResp.Conn.Write(data)
		log.Printf("Tried to write %d bytes, wrote %d bytes. Error: %v", len(data), n, err)
	}()

	ch := make(chan int)
	<-ch
}
