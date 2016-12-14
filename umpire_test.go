package umpire

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFiles(t *testing.T) {
	_, err := readFiles(map[string]io.Reader{
		"main.cpp": strings.NewReader("This is cool"),
		"main.h":   strings.NewReader("Fine this works"),
	})
	if err != nil {
		t.Fail()
	}
	//printStruct(out)
}

func TestReadSolution(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	dir := filepath.Join(gopath, "src/github.com/maddyonline/problems/problem-1")
	if _, err := ReadSolution(nil, dir); err != nil {
		t.Error(err)
	}
}

func TestRunDefault(t *testing.T) {
	gopath := os.Getenv("GOPATH")
	dir := filepath.Join(gopath, "src/github.com/maddyonline/problems")
	agent, err := NewAgent(nil, &dir, nil)
	if err != nil {
		t.Error(err)
	}
	p := &Payload{
		Problem: &Problem{
			Id: "problem-1",
		},
	}
	soln, err := ReadSolution(p, filepath.Join(dir, p.Problem.Id))
	if soln == nil {
		t.Fatalf("Got nil solution")
	}
	if err != nil {
		t.Error(err)
	}
	agent.Data[p.Problem.Id] = &JudgeData{
		Solution: soln,
	}
	p.Stdin = "Hello\nbye\n"
	resp := RunDefault(agent, p)
	printStruct(resp)
}

var raw = `{
  "prob-1": {
    "id": "prob-1",
    "title": "sort in decreasing order",
    "description": "Given an array, sort it in decreasing order",
    "io": [
      {
        "input": "hello\nhi\n",
        "output": "5\n2\n"
      },
      {
        "input": "hi\nhello\n",
        "output": "2\n5\n"
      }
    ],
    "tags": {
      "company": [
        "microsoft",
        "google"
      ],
      "difficulty": [
        "easy"
      ]
    },
    "solution": {
      "files": [
        {
          "Name": "main.py",
          "Content": "pass"
        }
      ],
      "language": "python",
      "stdin": ""
    }
  },
  "prob-2": {
    "id": "prob-2",
    "title": "sort in decreasing order",
    "description": "Given an array, sort it in decreasing order",
    "io": [
      {
        "input": "hello\nhi\n",
        "output": "5\n2\n"
      },
      {
        "input": "hi\nhello\n",
        "output": "2\n5\n"
      }
    ],
    "tags": {
      "company": [
        "microsoft",
        "google"
      ],
      "difficulty": [
        "easy"
      ]
    },
    "solution": {
      "files": [
        {
          "Name": "main.cpp",
          "Content": "# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"
        }
      ],
      "language": "cpp",
      "stdin": ""
    }
  }
}`

func TestDecoding(t *testing.T) {
	v := map[string]*JudgeData{}
	err := json.NewDecoder(strings.NewReader(raw)).Decode(&v)
	if err != nil {
		fmt.Printf("err=%v", err)
		t.Fail()
	}
	//fmt.Printf("%#v\n", v)
}

var rawpayload = `{"problem":{"id":"prob-2"},"language":"cpp","stdin":"here\nhellotherehowareyou\ncol\nteh\reallynice\ncurse\nof\ndimensionality\n","files":[{"name":"main.cpp","content":"# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"}]}`

func TestNewAgentExecution(t *testing.T) {
	agent, err := NewAgent(nil, nil, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	incoming := &Payload{}
	if err := json.NewDecoder(strings.NewReader(rawpayload)).Decode(incoming); err != nil {
		t.Errorf("Error: %v", err)
	}
	json.NewEncoder(os.Stdout).Encode(ExecuteDefault(agent, incoming))
}

func TestReadOne(t *testing.T) {
	UmpireCacheFilename = ".umpire_test.cache.json"
	data := map[string]*JudgeData{}
	gopath := os.Getenv("GOPATH")
	dir := filepath.Join(gopath, "src/github.com/maddyonline/problems/problem-1")
	if err := ReadOneProblem(data, "problem-122", dir); err != nil {
		t.Error(err)
	}
}

func TestReadAll(t *testing.T) {
	UmpireCacheFilename = ".umpire_test.cache.json"
	data := map[string]*JudgeData{}
	gopath := os.Getenv("GOPATH")
	dir := filepath.Join(gopath, "src/github.com/maddyonline/problems")
	if err := ReadAllProblems(data, dir); err != nil {
		t.Error(err)
	}
	UpdateCache(data)
}

func TestReadCache(t *testing.T) {
	UmpireCacheFilename = ".umpire_test.cache.json"
	data, err := ReadCache()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("data=%+v\n", data)
}
