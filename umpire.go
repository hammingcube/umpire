package umpire

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/docker/docker/client"
	"github.com/labstack/gommon/log"
	"io"
	"io/ioutil"
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

type JudgeData struct {
	Solution  *Payload
	Testcases []*TestCase
}

type Agent struct {
	Client      *client.Client
	ProblemsDir string
	Data        map[string]*JudgeData
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
		log.Printf("%s", tmpfn)
	}
	return &dir, nil
}

func (u *Agent) loadTestCases(problemsDir string, payload *Payload) ([]*TestCase, error) {
	if u.Data != nil && u.Data[payload.Problem.Id] != nil {
		return u.Data[payload.Problem.Id].Testcases, nil
	}
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
	testcases, err := u.loadTestCases(u.ProblemsDir, payload)
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

func (u *Agent) ReadSoln(payload *Payload) (*Payload, error) {
	log.Infof("ReadSoln: u.Data=%#v", u.Data)
	if u.Data != nil && u.Data[payload.Problem.Id] != nil {
		return u.Data[payload.Problem.Id].Solution, nil
	}
	dirname := filepath.Join(u.ProblemsDir, payload.Problem.Id, "solution")
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
	return &Payload{
		Language: lang,
		Files:    inMemoryFiles,
	}, nil
}

func (u *Agent) RunAndJudge(ctx context.Context, payload *Payload, stdout, stderr io.Writer) error {
	ctx, cancelFunc := context.WithCancel(ctx)

	solveCorrectSolution := func(ctx context.Context, ch chan error, w io.Writer) {
		payloadToSend, err := u.ReadSoln(payload)
		log.Infof("solveCorrectSolution: payloadToSend=%#v, err=%v", payloadToSend, err)
		if err != nil {
			ch <- err
			return
		}
		payloadToSend.Stdin = payload.Stdin
		ch <- DockerRun(ctx, u.Client, payloadToSend, w, ioutil.Discard)
		log.Info("solveCorrectSolution: Finished")
	}
	solveCurrentSolution := func(ctx context.Context, ch chan error, r io.Reader) {
		testcase := &TestCase{
			Input:    strings.NewReader(payload.Stdin),
			Expected: r,
		}
		ch <- u.JudgeTestcase(ctx, payload, stdout, stderr, testcase)
		log.Info("solveCurrentSolution: Finished")
	}

	ch := make(chan error)
	r, w := io.Pipe()

	go solveCorrectSolution(ctx, ch, w)
	go solveCurrentSolution(ctx, ch, r)

	err := <-ch
	if err != nil {
		cancelFunc()
		go func() {
			err := <-ch
			log.Infof("Btw, second err=%v", err)
		}()
		return err
	}
	return <-ch
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
