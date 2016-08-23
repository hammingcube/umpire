package umpire

import (
	"bufio"
	"bytes"
	"errors"
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

type Agent struct {
	Client      *client.Client
	ProblemsDir string
}

type Decision string

const (
	Fail Decision = "fail"
	Pass          = "pass"
)

type Response struct {
	Status  Decision `json:"status"`
	Details string   `json:"details"`
	Stdout  string   `json:"stdout", omitempty`
	Stderr  string   `json:"stderr", omitempty`
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

func (u *Agent) JudgeTestcase(ctx context.Context, payload *Payload, stdout, stderr io.Writer, testcase *TestCase) error {
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
	return DockerJudge(ctx, u.Client, payloadToSend, stdout, stderr, bufio.NewScanner(testcase.Expected))
}

func (u *Agent) JudgeAll(ctx context.Context, payload *Payload, stdout, stderr io.Writer) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	errors := make(chan error)
	testcases, err := loadTestCases(u.ProblemsDir, payload)
	if err != nil {
		return err
	}
	for i, testcase := range testcases {
		wg.Add(1)
		go func(ctx context.Context, i int, testcase *TestCase) {
			defer wg.Done()
			err := u.JudgeTestcase(ctx, payload, ioutil.Discard, ioutil.Discard, testcase)
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

func readFiles(files map[string]io.Reader) ([]*InMemoryFile, error) {
	ans := []*InMemoryFile{}
	for filename, file := range files {
		data, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		ans = append(ans, &InMemoryFile{filename, string(data)})
	}
	return ans, nil
}

func ReadSoln(dirname string) (*Payload, error) {
	supported := map[string]bool{"cpp": true}
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	var lang string
	for _, file := range files {
		if file.IsDir() && supported[file.Name()] {
			lang = file.Name()
			break
		}
	}
	if lang == "" {
		return nil, errors.New("Not Found")
	}

	dir := filepath.Join(dirname, lang)
	srcFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	toRead := map[string]io.Reader{}
	toClose := []io.ReadCloser{}
	defer func() {
		for _, f := range toClose {
			f.Close()
		}
	}()
	for _, srcFile := range srcFiles {
		if !srcFile.IsDir() {
			f, err := os.Open(filepath.Join(dir, srcFile.Name()))
			if err != nil {
				return nil, err
			}
			toClose = append(toClose, f)
			toRead[srcFile.Name()] = f
		}
	}
	inMemoryFiles, err := readFiles(toRead)
	if err != nil {
		return nil, err
	}
	payload := &Payload{
		Language: lang,
		Files:    inMemoryFiles,
	}
	return payload, nil
}

func (u *Agent) RunAndJudge(ctx context.Context, payload *Payload, stdout, stderr io.Writer) error {
	r, w := io.Pipe()
	correctlySolve := func(w io.Writer) {
		payloadToSend, err := ReadSoln(filepath.Join(u.ProblemsDir, payload.Problem.Id, "solution"))
		if err != nil {
			return
		}
		payloadToSend.Stdin = payload.Stdin
		DockerRun(ctx, u.Client, payloadToSend, w, ioutil.Discard)
	}

	go correctlySolve(w)

	testcase := &TestCase{
		Input:    strings.NewReader(payload.Stdin),
		Expected: r,
	}
	return u.JudgeTestcase(context.Background(), payload, stdout, stderr, testcase)
}

func JudgeDefault(u *Agent, payload *Payload) *Response {
	err := u.JudgeAll(context.Background(), payload, ioutil.Discard, ioutil.Discard)
	if err != nil {
		return &Response{
			Status:  Fail,
			Details: err.Error(),
		}
	}
	return &Response{
		Status: Pass,
	}
}

func RunDefault(u *Agent, payload *Payload) *Response {
	var stdout, stderr bytes.Buffer
	err := u.RunAndJudge(context.Background(), payload, &stdout, &stderr)
	if err != nil {
		return &Response{Fail, err.Error(), stdout.String(), stderr.String()}
	}
	return &Response{Pass, "Output is as expected", stdout.String(), stderr.String()}
}
