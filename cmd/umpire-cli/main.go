package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/maddyonline/umpire"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const CPP_CODE = `# include <iostream>
# include <chrono>
# include <thread>


using namespace std;
int main() {
  string s;
  while(cin >> s) {
  	std::this_thread::sleep_for(std::chrono::milliseconds(5));
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
	Stdin: "here\nhellotherehowareyou\ncol\nteh\reallynice\n",
}

func exampleDockerRun() error {
	cli, _ := client.NewEnvClient()
	err := umpire.DockerRun(context.Background(), cli, payloadExample, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	return err
}

func exampleDockerJudge() error {
	cli, _ := client.NewEnvClient()
	expected := strings.NewReader("5\n2\n")
	err := umpire.DockerJudge(context.Background(), cli, payloadExample, os.Stdout, ioutil.Discard, bufio.NewScanner(expected))
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	return err
}

func exampleDockerJudgeMulti() error {
	cli, _ := client.NewEnvClient()
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

func exampleRunAndJudge() {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	problemsDir, err := filepath.Abs("../../")
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	u := &umpire.Agent{cli, problemsDir, nil}
	err = u.RunAndJudge(context.Background(), payloadExample, os.Stdout, ioutil.Discard)
	log.Printf("In main, got: %v", err)
}

func exampleJudgeAll() {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	problemsDir, err := filepath.Abs("../../")
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	u := &umpire.Agent{cli, problemsDir, nil}
	err = u.JudgeAll(context.Background(), payloadExample, ioutil.Discard, ioutil.Discard)
	log.Printf("In main, got: %v", err)
}

func run() {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	problemsDir, err := filepath.Abs("../../")
	if err != nil {
		log.Fatalf("%v", err)
		return
	}
	u := &umpire.Agent{cli, problemsDir, nil}
	out := umpire.RunDefault(u, payloadExample)
	fmt.Printf("out=%v\n", out)
}

func main() {
	run()
	//exampleDockerRun()
	//exampleDockerJudge()
	//log.Printf("In main: %v", exampleDockerJudgeMulti())

	//exampleRunAndJudge()

	// //err = u.JudgeAll(context.Background(), payloadExample, ioutil.Discard, ioutil.Discard)

}
