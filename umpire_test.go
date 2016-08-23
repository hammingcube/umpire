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

func TestReadSoln(t *testing.T) {
	out, err := ReadSoln("/Users/madhavjha/src/github.com/maddyonline/problems/problem-1/solution")
	if err != nil {
		t.Fail()
	}
	printStruct(out)
}
