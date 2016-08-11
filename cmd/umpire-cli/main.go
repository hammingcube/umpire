package main

import (
	"bytes"
	"fmt"
	"github.com/docker/engine-api/client"
	"github.com/maddyonline/umpire"
	"os"
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
	err := umpire.DockerRun(cli, payloadExample, os.Stdout)
	if err != nil {
		fmt.Printf("%v", err)
	}
}

func exampleRun() {
	var b1, b2 bytes.Buffer
	umpire.Run(payloadExample, &b1, &b2)
	fmt.Printf("%s", b1.String())
	//umpire.Judge(payloadExample)
}

func main() {
	//exampleDockerRun()
	umpire.Judge(payloadExample)
}
