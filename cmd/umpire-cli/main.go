package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/docker/engine-api/client"
	"github.com/maddyonline/umpire"
	"io/ioutil"
	"os"
	"strings"
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

func exampleDockerRun() {
	cli, _ := client.NewEnvClient()
	err := umpire.DockerRun(cli, payloadExample, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Printf("%v", err)
	}
}

func exampleDockerJudge() {
	cli, _ := client.NewEnvClient()
	expected := strings.NewReader("5\n2\n")
	err := umpire.DockerJudge(cli, payloadExample, os.Stdout, ioutil.Discard, bufio.NewScanner(expected))
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}

func exampleRun() {
	var b2 bytes.Buffer
	result := umpire.Run(payloadExample, os.Stdout, &b2)
	fmt.Printf("result: %v\n", result)
	//fmt.Printf("stderr: %q\n", b2.String())
	//umpire.Judge(payloadExample)
}

func main() {
	exampleDockerJudge()
	//exampleRun()
	//umpire.Judge(payloadExample)
}
