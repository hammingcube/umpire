package main

import (
	"encoding/json"
	"github.com/docker/engine-api/client"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

const basic_example = `{
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

type Payload struct {
	Language string          `json:"language"`
	Files    []*InMemoryFile `json:"files"`
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

func Evaluate(cli *client.Client, payload *Payload, testcase io.Reader) error {
	workDir, err := createDirectoryWithFiles(payload.Files)
	dieOnErr(err)
	// defer os.RemoveAll(*workDir)
	srcDir, err := filepath.Abs(*workDir)
	dieOnErr(err)
	log.Println(srcDir)
	result, err := DockerEval(cli, srcDir, payload.Language, testcase)
	if err != nil {
		return err
	}
	defer func() {
		if err := result.cleanup(); err != nil {
			log.Fatal("Error cleaning up container: %v", err)
		}
	}()
	go func() {
		_, err = io.Copy(os.Stdout, result.reader)
		if err != nil && err != io.EOF {
			log.Fatal(err)
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
	cli, err := client.NewEnvClient()
	payload := &Payload{}
	err = json.Unmarshal([]byte(basic_example), payload)
	dieOnErr(err)
	testcase, err := os.Open("input-new.txt")
	dieOnErr(err)
	Evaluate(cli, payload, testcase)
}
