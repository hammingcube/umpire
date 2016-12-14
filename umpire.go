package umpire

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/labstack/gommon/log"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
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
	printStruct(u)
	fmt.Println("---")
	printStruct(payload)
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
		log.Infof("Read file %s", filename)
		ans = append(ans, &InMemoryFile{filename, string(data)})
	}
	return ans, nil
}

func (u *Agent) Execute(ctx context.Context, incoming *Payload, stdout, stderr io.Writer) error {
	return DockerRun(ctx, u.Client, incoming, stdout, stderr)
}

func (u *Agent) RunAndJudge(ctx context.Context, incoming *Payload, stdout, stderr io.Writer) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	if u.Data == nil {
		return fmt.Errorf("Umpire agent's data not initialized: %v", u.Data)
	}
	if u.Data[incoming.Problem.Id] == nil {
		return fmt.Errorf("Problem Id '%s' not found", incoming.Problem.Id)
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

func ExecuteDefault(u *Agent, incoming *Payload) *Response {
	var stdout, stderr bytes.Buffer
	err := u.Execute(context.Background(), incoming, &stdout, &stderr)
	log.Printf("Execute: %#v", err)
	if err != nil {
		return &Response{Fail, err.Error(), stdout.String(), stderr.String()}
	}
	if stderr.String() != "" {
		return &Response{Fail, "Error while running program", stdout.String(), stderr.String()}
	}
	return &Response{Pass, "", stdout.String(), stderr.String()}
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

func fetchProblems(apiUrl string) (map[string]*JudgeData, error) {
	url := fmt.Sprintf("%s/problems", apiUrl)
	log.Infof("Sending request to %s", url)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("content-type", "application/json")
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	v := map[string]*JudgeData{}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return nil, err
	}
	log.Infof("fetchProblems: number of problems=%d", len(v))
	return v, nil
}

func prepareAgent(agent *Agent, values map[string]string) error {
	if agent.Client == nil {
		cli, err := client.NewEnvClient()
		if err != nil {
			return err
		}
		agent.Client = cli
		log.Info("Successfully initialized docker client")
	}

	if values == nil {
		return nil
	}
	if serverdb, ok := values["serverdb"]; ok {
		log.Infof("Fetching problems from %s", serverdb)
		data, err := fetchProblems(serverdb)
		if err != nil {
			return err
		}
		agent.Data = data
	}
	if problemsdir, ok := values["problemsdir"]; ok {
		problemsDir, err := filepath.Abs(problemsdir)
		if err != nil {
			return err
		}
		agent.ProblemsDir = problemsDir
		log.Infof("Using `%s` as problems directory", problemsDir)
		return nil
	}
	return nil
}

func printStruct(v interface{}) {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))
}

func NewAgent(cli *client.Client, data map[string]*JudgeData) (*Agent, error) {
	if cli == nil {
		dcli, err := client.NewEnvClient()
		if err != nil {
			return nil, err
		}
		cli = dcli
	}
	return &Agent{
			Client: cli,
			Data:   data,
		},
		nil
}

var UmpireCacheFilename = ".umpire.cache.json"
var LangPriority = map[string]int{"cpp": 1, "python": 2, "javascript": 3, "typescript": 4}

const SOLUTION_DIR = "solution"

type LangDir []struct {
	priority int
	name     string
}

func (a LangDir) Len() int           { return len(a) }
func (a LangDir) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a LangDir) Less(i, j int) bool { return a[i].priority < a[j].priority }

func ReadSolution(payload *Payload, solutionsDir string) (*Payload, error) {
	files, err := ioutil.ReadDir(filepath.Join(solutionsDir, SOLUTION_DIR))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	langDirs := LangDir{}
	for _, f := range files {
		if f.IsDir() && LangPriority[f.Name()] != 0 {
			langDirs = append(langDirs, struct {
				priority int
				name     string
			}{LangPriority[f.Name()], f.Name()})
		}
	}
	if len(langDirs) < 1 {
		return nil, fmt.Errorf("No solution found")
	}
	sort.Sort(langDirs)
	language := langDirs[0].name
	log.Infof("Using %s language for solution", language)
	srcDir := filepath.Join(solutionsDir, "solution", language)
	srcFiles, err := ioutil.ReadDir(srcDir)
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
			f, err := os.Open(filepath.Join(srcDir, srcFile.Name()))
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
	if payload == nil {
		payload = &Payload{}
	}
	payload.Language = language
	payload.Files = inMemoryFiles
	return payload, nil
}

func getCacheFilename() string {
	prefix := ""
	if user, err := user.Current(); err == nil {
		prefix = user.HomeDir
	}
	return filepath.Join(prefix, UmpireCacheFilename)
}

func ReadCache() (map[string]*JudgeData, error) {
	data := make(map[string]*JudgeData)
	cacheFile, err := os.Open(getCacheFilename())
	if err != nil {
		return data, err
	}
	defer cacheFile.Close()
	if err := json.NewDecoder(cacheFile).Decode(&data); err != nil {
		return data, err
	}
	return data, nil
}

func ReadOneProblem(data map[string]*JudgeData, problemId, solutionsDir string) error {
	if data == nil {
		return nil
	}
	solution, err := ReadSolution(nil, solutionsDir)
	if err != nil {
		return err
	}
	if solution != nil {
		data[problemId] = &JudgeData{
			Solution: solution,
		}
	}
	return nil
}

func ReadAllProblems(data map[string]*JudgeData, problemsDir string) error {
	files, err := ioutil.ReadDir(problemsDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if err := ReadOneProblem(data, f.Name(), filepath.Join(problemsDir, f.Name())); err != nil {
			return err
		}
	}
	return nil
}

func UpdateCache(data map[string]*JudgeData) error {
	if data == nil {
		return nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(data)
	if err != nil {
		return err
	}
	cacheFile, err := os.Create(getCacheFilename())
	if err != nil {
		log.Warnf("Failed to update cache file: %v", err)
		log.Infof("Please udpate %s with following content: %s", getCacheFilename(), b.String())
		return err
	}
	defer cacheFile.Close()
	cacheFile.Write(b.Bytes())
	log.Info("Updated cache")
	return nil
}
