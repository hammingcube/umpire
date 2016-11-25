package umpire

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func printStruct(v interface{}) {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))

}

func TestReadFiles(t *testing.T) {
	out, err := readFiles(map[string]io.Reader{
		"main.cpp": strings.NewReader("This is cool"),
		"main.h":   strings.NewReader("Fine this works"),
	})
	if err != nil {
		t.Fail()
	}
	printStruct(out)
}

// func TestReadSoln(t *testing.T) {
// 	u := &Agent{}
// 	out, err := u.ReadSoln("/Users/madhavjha/src/github.com/maddyonline/problems/problem-1/solution")
// 	if err != nil {
// 		t.Fail()
// 	}
// 	printStruct(out)
// }

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

var newpayload = `{"problem":{"id":"prob-2"},"language":"cpp","stdin":"here\nhellotherehowareyou\ncol\nteh\reallynice\ncurse\nof\ndimensionality\n","files":[{"name":"main.cpp","content":"# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"}]}`

func TestDecoding(t *testing.T) {
	v := map[string]*JudgeData{}
	err := json.NewDecoder(strings.NewReader(raw)).Decode(&v)
	if err != nil {
		fmt.Printf("err=%v", err)
		t.Fail()
	}
	fmt.Printf("%#v\n", v)
}
