package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/maddyonline/umpire"
	"github.com/maddyonline/umpire/pkg/dockerutils"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const CPP_CODE = `
# include <iostream>
# include <chrono>

using namespace std;
int main() {
  string s;
  while(cin >> s) {
    cout << s.size() << endl;
  }
}`

var payloadExample = &umpire.Payload{
	Problem:  &umpire.Problem{"maddyonline/problems/problem-1"},
	Language: "cpp",
	Files: []*umpire.InMemoryFile{
		&umpire.InMemoryFile{
			Name:    "main.cpp",
			Content: CPP_CODE,
		},
	},
	Stdin: "here\nhellotherehowareyou\ncol\nteh\nreallynice\n",
}

func TestDockerRun(t *testing.T) {
	var b bytes.Buffer
	cli := dockerutils.NewClient()
	if cli == nil {
		t.Errorf("Failed to initialize docker client")
	}
	if err := umpire.DockerRun(context.Background(), cli, payloadExample, &b, os.Stderr); err != nil {
		t.Error(err)
	}
	expected := "4\n19\n3\n3\n10\n"
	if b.String() != expected {
		t.Errorf("got: %q, expected: %q", b.String(), expected)
	}
}

func TestDockerJudge(t *testing.T) {
	cli := dockerutils.NewClient()
	if cli == nil {
		t.Errorf("Failed to initialize docker client")
	}
	expected := strings.NewReader("4\n19\n3\n3\n10\n")
	if err := umpire.DockerJudge(context.Background(), cli, payloadExample, os.Stdout, ioutil.Discard, bufio.NewScanner(expected)); err != nil {
		t.Error(err)
	}

}

func exampleDockerJudgeMulti() error {
	cli := dockerutils.NewClient()
	if cli == nil {
		return fmt.Errorf("Failed to initialize docker client")
	}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	errors := make(chan error)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(ctx context.Context, i int) {
			defer wg.Done()
			expected := strings.NewReader("5\n2\n")

			if i == 4 {
				log.Printf("i: %d", i)
				expected = strings.NewReader("4\n2\n")
			}
			select {
			case errors <- umpire.DockerJudge(ctx, cli, payloadExample, os.Stdout, ioutil.Discard, bufio.NewScanner(expected)):
			case <-ctx.Done():
				log.Printf("Context has been cancelled, returning")
			}
		}(ctx, i)
	}
	go func() {
		wg.Wait()
		close(errors)
	}()
	//sum := 0
	var finalErr error
	var fail int
	for err := range errors {
		if err != nil {
			fail += 1
			if !strings.Contains(err.Error(), "Context cancelled") {
				finalErr = err
			}
		}
		if err != nil && strings.Contains(err.Error(), "Mismatch") {
			cancel()
		}
	}
	log.Printf("Fail: %d", fail)
	return finalErr
	//log.Printf("successes: %d", sum)
}

func exampleRunAndJudge() error {
	cli := dockerutils.NewClient()
	if cli == nil {
		return fmt.Errorf("Failed to initialize docker client")
	}
	problemsDir, err := filepath.Abs("../../")
	if err != nil {
		log.Fatalf("%v", err)
		return err
	}
	u := &umpire.Agent{cli, problemsDir, nil}
	err = u.RunAndJudge(context.Background(), payloadExample, os.Stdout, ioutil.Discard)
	log.Printf("In main, got: %v", err)
	return nil
}

func exampleJudgeAll() error {
	cli := dockerutils.NewClient()
	if cli == nil {
		return fmt.Errorf("Failed to initialize docker client")
	}

	problemsDir, err := filepath.Abs("../../")
	if err != nil {
		log.Fatalf("%v", err)
		return err
	}
	u := &umpire.Agent{cli, problemsDir, nil}
	err = u.JudgeAll(context.Background(), payloadExample, ioutil.Discard, ioutil.Discard)
	log.Printf("In main, got: %v", err)
	return err
}

func run() error {
	cli := dockerutils.NewClient()
	if cli == nil {
		return fmt.Errorf("Failed to initialize docker client")
	}
	problemsDir, err := filepath.Abs("../../")
	if err != nil {
		log.Fatalf("%v", err)
		return err
	}
	u := &umpire.Agent{cli, problemsDir, nil}
	out := umpire.RunDefault(u, payloadExample)
	fmt.Printf("out=%v\n", out)
	return nil
}

func newmain() {
	run()
	//exampleDockerRun()
	//exampleDockerJudge()
	//log.Printf("In main: %v", exampleDockerJudgeMulti())

	//exampleRunAndJudge()

	// //err = u.JudgeAll(context.Background(), payloadExample, ioutil.Discard, ioutil.Discard)

}
