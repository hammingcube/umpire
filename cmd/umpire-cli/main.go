package main

import (
	"bufio"
	"fmt"
	"github.com/docker/engine-api/client"
	"github.com/maddyonline/umpire"
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"os"
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
  	std::this_thread::sleep_for(std::chrono::milliseconds(10));
    cout << s.size() << endl;
  }
}`

var payloadExample = &umpire.Payload{
	Problem:  &umpire.Problem{"problem-1"},
	Language: "cpp",
	Files: []*umpire.InMemoryFile{
		&umpire.InMemoryFile{
			Name:    "main.cpp",
			Content: CPP_CODE,
		},
	},
	Stdin: "hello\nhi\n",
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

func main() {
	//exampleDockerRun()
	//exampleDockerJudge()
	//log.Printf("In main: %v", exampleDockerJudgeMulti())

	//exampleRun()
	cli, _ := client.NewEnvClient()
	//umpire.JudgeAll(context.Background(), cli, payloadExample, ioutil.Discard, ioutil.Discard)
	umpire.RunClient(context.Background(), cli, payloadExample, os.Stdout, os.Stderr)
}
