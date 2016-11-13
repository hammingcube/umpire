package umpire

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"testing"
)

func TestDocker(t *testing.T) {
	cli, err := docker.NewEnvClient()
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		fmt.Printf("%s %s\n", container.ID[:10], container.Image)
	}
}
