package umpire

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/labstack/gommon/log"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type TestCase struct {
	Input    io.Reader `json:"input"`
	Expected io.Reader `json:"output"`
	Id       string
}

type JudgeData struct {
	Solution *Payload `json:"solution"`
	IO       []*struct {
		Input  string `json:"input"`
		Output string `json:"output"`
	} `json:"io"`
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
		testcases := []*TestCase{}
		for _, io := range u.Data[payload.Problem.Id].IO {
			testcases = append(testcases, &TestCase{strings.NewReader(io.Input), strings.NewReader(io.Output), payload.Problem.Id})
		}
		return testcases, nil
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
	log.Infof("JudgeTestcase: %#v, %v", testcaseData, err)

	if err != nil {
		return err
	}
	payloadToSend := &Payload{}
	*payloadToSend = *payload
	payloadToSend.Stdin = string(testcaseData)
	return DockerJudge(ctx, u.Client, payloadToSend, stdout, stderr, bufio.NewScanner(testcase.Expected))
}

func (u *Agent) UpdateProblemsCache(jd *JudgeData) (string, error) {
	if u.Data == nil {
		return "", fmt.Errorf("Agent's data map not initialized")
	}
	key := RandStringRunes(12)
	u.Data[key] = jd
	return key, nil
}

func (u *Agent) RemoveFromProblemsCache(key string) {
	if u.Data == nil {
		return
	}
	if _, ok := u.Data[key]; ok {
		delete(u.Data, key)
	}
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
	log.Infof("ReadSoln: number of problems = %d", len(u.Data))
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

func (u *Agent) RunAndJudge(ctx context.Context, incoming *Payload, stdout, stderr io.Writer) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	if u.Data == nil || u.Data[incoming.Problem.Id] == nil {
		return errors.New("Solution not found")
	}
	log.Info("Found correct solution for problem %s", incoming.Problem.Id)
	solnPayload := u.Data[incoming.Problem.Id].Solution
	solnPayload.Stdin = incoming.Stdin

	ch := make(chan error)
	r, w := io.Pipe()

	testcase := &TestCase{
		Input:    strings.NewReader(incoming.Stdin),
		Expected: r,
	}
	go func(ctx context.Context, payload *Payload, ch chan error) {
		defer w.Close()
		var stderr bytes.Buffer
		err := DockerRun(ctx, u.Client, payload, w, &stderr)
		if err != nil {
			ch <- err
			return
		}
		if stderr.String() != "" {
			ch <- errors.New(stderr.String())
			return
		}
		log.Info("Done solving correct solution")
		ch <- nil

	}(ctx, solnPayload, ch)

	go func(ctx context.Context, payload *Payload, ch chan error) {
		defer r.Close()
		ch <- u.JudgeTestcase(ctx, payload, stdout, stderr, testcase)
		log.Info("Done solving user solution")
	}(ctx, incoming, ch)

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

func RunDefault(u *Agent, incoming *Payload) *Response {
	var stdout, stderr bytes.Buffer
	err := u.RunAndJudge(context.Background(), incoming, &stdout, &stderr)
	log.Printf("RunDefault: %#v", err)
	if err != nil {
		return &Response{Fail, err.Error(), stdout.String(), stderr.String()}
	}
	return &Response{Pass, "Output is as expected", stdout.String(), stderr.String()}
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
