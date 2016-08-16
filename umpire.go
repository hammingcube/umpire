package umpire

import (
	"bufio"
	"encoding/json"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	_ "time"
)

type TestCase struct {
	Input    io.Reader
	Expected io.Reader
	Id       string
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

func loadTestCases(problemsDir string, payload *Payload) ([]*TestCase, error) {
	testcases := []*TestCase{}
	files, err := ioutil.ReadDir(filepath.Join(problemsDir, payload.Problem.Id, "testcases"))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if strings.Contains(file.Name(), "input") {
			inputFilename := file.Name()
			expectedFilename := strings.Replace(file.Name(), "input", "output", 1)
			input, err := os.Open(filepath.Join(problemsDir, payload.Problem.Id, "testcases", inputFilename))
			if err != nil {
				return nil, err
			}
			expected, err := os.Open(filepath.Join(problemsDir, payload.Problem.Id, "testcases", expectedFilename))
			if err != nil {
				return nil, err
			}
			testcases = append(testcases, &TestCase{input, expected, inputFilename})
		}
	}
	return testcases, nil
}

func RunIt(ctx context.Context, cli *client.Client, payload *Payload, stdout, stderr io.Writer) error {
	return DockerRun(ctx, cli, payload, stdout, stderr)
}

func JudgeIt(ctx context.Context, cli *client.Client, payload *Payload, stdout, stderr io.Writer, testcase *TestCase) error {
	workDir, err := createDirectoryWithFiles(payload.Files)
	if err != nil {
		return err
	}
	defer func() {
		log.Printf("Removing temporary working directory: %s", *workDir)
		os.RemoveAll(*workDir)
	}()
	testcaseData, err := ioutil.ReadAll(testcase.Input)
	if err != nil {
		return err
	}
	payloadToSend := &Payload{}
	*payloadToSend = *payload
	payloadToSend.Stdin = string(testcaseData)
	return DockerJudge(ctx, cli, payloadToSend, stdout, stderr, bufio.NewScanner(testcase.Expected))
}

func JudgeAll(ctx context.Context, cli *client.Client, payload *Payload, stdout, stderr io.Writer) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	errors := make(chan error)
	problemsDir := "/Users/madhavjha/src/github.com/maddyonline/problems"
	testcases, err := loadTestCases(problemsDir, payload)
	if err != nil {
		return err
	}
	for i, testcase := range testcases {
		wg.Add(1)
		go func(ctx context.Context, i int, testcase *TestCase) {
			defer wg.Done()
			err := JudgeIt(ctx, cli, payload, ioutil.Discard, ioutil.Discard, testcase)
			log.Printf("testcase %d: %v", i, err)
			if err != nil {
				cancel()
			}
			errors <- err
		}(ctx, i, testcase)
	}
	go func() {
		wg.Wait()
		close(errors)
	}()
	var finalErr error
	var fail int
	for err := range errors {
		if err != nil {
			fail += 1
			if !strings.Contains(err.Error(), "Context cancelled") {
				finalErr = err
			}
		}
		// if err != nil && strings.Contains(err.Error(), "Mismatch") {
		// 	cancel()
		// }
	}
	log.Printf("Fail: %d", fail)
	return finalErr
}

func RunClient(ctx context.Context, cli *client.Client, payload *Payload, stdout, stderr io.Writer) error {
	r, w := io.Pipe()
	go func(w io.Writer) {
		problemsDir := "/Users/madhavjha/src/github.com/maddyonline/problems"
		data, err := ioutil.ReadFile(filepath.Join(problemsDir, payload.Problem.Id, "solution.json"))
		if err != nil {
			return
		}
		payload := &Payload{}
		err = json.Unmarshal(data, payload)
		if err != nil {
			return
		}
		RunIt(context.Background(), cli, payload, w, ioutil.Discard)
	}(w)
	testcases := []*TestCase{&TestCase{
		Input:    strings.NewReader(payload.Stdin),
		Expected: r,
	}}
	return JudgeIt(context.Background(), cli, payload, stdout, stderr, testcases[0])
}
