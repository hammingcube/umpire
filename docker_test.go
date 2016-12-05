package umpire

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/labstack/gommon/log"
	"io/ioutil"
	"testing"
)

var TurnOffLogging = true

func init() {
	if TurnOffLogging {
		log.SetOutput(ioutil.Discard)
	}
}

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

func TestWriteLine(t *testing.T) {
	var tests = []struct {
		input    string
		expected string
	}{
		{
			"hello",
			"hello\n",
		},
	}
	for _, test := range tests {
		var out bytes.Buffer
		if err := writeLine(&out, test.input); err != nil {
			t.Error("writeLine:", err)
		}
		got := out.String()
		if got != test.expected {
			t.Errorf("writeLine: expected %q got %q", test.expected, got)
		}
	}

}

func TestWriteConnSimple(t *testing.T) {
	var out bytes.Buffer
	if err := writeConn(&out, []byte("hello")); err != nil {
		t.Error("writeConn:", err)
	}
	if out.String() != "hello" {
		t.Errorf("writeConn: mismatched outputs, expected=%s, got=%s", "hello", out.String())
	}
}

func TestWriteConnLarge(t *testing.T) {
	b, err := ioutil.ReadFile("docker.go")
	if err != nil {
		t.Error(err)
	}
	var out bytes.Buffer
	if err := writeConn(&out, b); err != nil {
		t.Error("writeConn:", err)
	}
	if len(b) != len(out.Bytes()) {
		t.Errorf("writeConn: mismatched outputs, expected-len=%d, got-len=%d", len(b), len(out.Bytes()))
	}
}
