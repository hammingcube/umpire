package main

import (
	"bufio"
	"encoding/json"
	"github.com/docker/engine-api/client"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const basic_example = `{
  "problem": {"id": "problem-1"},
  "language": "cpp",
  "files": [
    {
      "name": "main.cpp",
      "content": "# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"
    }, {
    "name": "_stdin_",
    "content": "abc\nhello\n"
    }
  ]
}`

func dieOnErr(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

type Problem struct {
	Id string `json:"id"`
}

type TestCase struct {
	Input    io.Reader
	Expected io.Reader
}

type Payload struct {
	Language string          `json:"language"`
	Files    []*InMemoryFile `json:"files"`
	Problem  *Problem        `json:"problem"`
}

type InMemoryFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Result struct {
	Status string `json:"status"`
}

func createDirectoryWithFiles(files []*InMemoryFile) (*string, error) {
	dir, err := ioutil.TempDir(".", "work_dir_")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		tmpfn := filepath.Join(dir, file.Name)
		if err := ioutil.WriteFile(tmpfn, []byte(file.Content), 0666); err != nil {
			return nil, err
		}
		log.Println(tmpfn)
	}
	return &dir, nil
}

func Evaluate(cli *client.Client, payload *Payload, testcases []*TestCase) error {
	workDir, err := createDirectoryWithFiles(payload.Files)
	dieOnErr(err)
	defer os.RemoveAll(*workDir)
	srcDir, err := filepath.Abs(*workDir)
	dieOnErr(err)
	log.Println(srcDir)
	result, err := DockerEval(cli, srcDir, payload.Language, testcases[0].Input)
	if err != nil {
		return err
	}
	defer func() {
		if err := result.cleanup(); err != nil {
			log.Fatal("Error cleaning up container: %v", err)
		}
	}()
	go func() {
		scanner1 := bufio.NewScanner(result.reader)
		scanner2 := bufio.NewScanner(testcases[0].Expected)
		for scanner1.Scan() {
			scanner2.Scan()
			log.Printf("output: %s, expected: %s", scanner1.Text(), scanner2.Text())
			for _, scanner := range []*bufio.Scanner{scanner1, scanner2} {
				if err := scanner.Err(); err != nil {
					log.Fatal(err)
				}
			}
		}
	}()
	for {
		select {
		case <-result.done:
			log.Println("Done!")
			return nil
		case <-time.After(2 * time.Second):
			log.Println("Still going...")
			// result.cancel()
		}
	}

	return nil
}

func main() {
	problemsDir := "/Users/madhavjha/src/github.com/maddyonline/problems"
	cli, err := client.NewEnvClient()
	payload := &Payload{}
	err = json.Unmarshal([]byte(basic_example), payload)
	log.Printf("%v", payload.Problem)

	testcases := []*TestCase{}

	files, err := ioutil.ReadDir(filepath.Join(problemsDir, payload.Problem.Id, "testcases"))
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {

		if strings.Contains(file.Name(), "input") {
			inputFilename := file.Name()
			expectedFilename := strings.Replace(file.Name(), "input", "output", 1)
			input, err := os.Open(filepath.Join(problemsDir, payload.Problem.Id, "testcases", inputFilename))
			dieOnErr(err)
			expected, err := os.Open(filepath.Join(problemsDir, payload.Problem.Id, "testcases", expectedFilename))
			dieOnErr(err)
			testcases = append(testcases, &TestCase{input, expected})
		}
	}
	Evaluate(cli, payload, testcases)
}
