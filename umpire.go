package umpire

import (
	// "bufio"
	"encoding/json"
	"errors"
	"fmt"
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

const basic_example = `{
  "problem": {"id": "problem-1"},
  "language": "cpp",
  "stdin": "hello\nhi\n",
  "files": [
    {
      "name": "main.cpp",
      "content": "# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"
    }
  ]
}`

func dieOnErr(err error) {
	if err != nil {
		log.Println(err)
		//os.Exit(1)
	}
}

type TestCase struct {
	Input      io.Reader
	Expected   io.Reader
	Id         string
	fromDocker bool
}

type RunStatus string

const (
	Pass = "Pass"
	Fail = "Fail"
)

type Result struct {
	Status  RunStatus `json:"status"`
	Details string    `json:"details"`
}

type ErrKnown struct {
	Type      string
	ShortDesc string
	LongDesc  string
	Err       error
}

func (v ErrKnown) Error() string {
	return fmt.Sprintf("%s: %s", v.Type, v.ShortDesc)
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

type ErrMismatch struct {
}

func (v ErrMismatch) Error() string {
	return "Mismatched"
}

func SpecialWrite(w io.Writer, text string) error {
	n, err := w.Write([]byte(text + "\n"))
	if n != len(text)+1 || (err != nil && err != io.EOF) {
		errorMsg := fmt.Sprintf("Error while writing %d bytes, wrote only %d bytes. Err: %v", len(text)+1, n, err)
		return errors.New(errorMsg)
	}
	return nil
}

func evaluate(ctx context.Context, cli *client.Client, payload *Payload, testNum int, testcase *TestCase, stdoutWriter, stderrWriter io.Writer) error {
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
	dockerEvalResult, err := dockerEval(ctx, cli, payloadToSend)
	if err != nil {
		return err
	}
	defer func() {
		if err := dockerEvalResult.Cleanup(); err != nil {
			log.Fatal("Error cleaning up container: %v", err)
		}
	}()
	go func() {
		io.Copy(os.Stdout, dockerEvalResult.Stdout)
	}()
	// ch := make(chan struct{})
	// go func() {
	// 	scanner := bufio.NewScanner(r)
	// 	for scanner.Scan() {
	// 		fmt.Println(scanner.Text())
	// 		if scanner.Err() != nil {
	// 			fmt.Printf("Scanner Err: %v", scanner.Err())
	// 		}
	// 	}
	// 	ch <- struct{}{}
	// }()

	<-dockerEvalResult.Done
	//<-ch
	//fmt.Printf("b: %q\n", b.String())
	return ErrKnown{}

}

func loadTestCases(problemsDir string, payload *Payload) []*TestCase {
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
			testcases = append(testcases, &TestCase{input, expected, inputFilename, false})
		}
	}
	return testcases
}

func evaluateAll(cli *client.Client, payload *Payload, testcases []*TestCase, stdout, stderr io.Writer) ErrKnown {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	errorChan := make(chan error)
	for i, testcase := range testcases {
		wg.Add(1)
		go func(i int, testcase *TestCase) {
			defer func() {
				log.Printf("Done evaluating testcase %d", i)
				wg.Done()
			}()
			err := evaluate(ctx, cli, payload, i, testcase, stdout, stderr)
			if err != nil {
				log.Printf("In evaluateAll, evaluate error: %v", err)
			}
			errorChan <- err
		}(i, testcase)
	}
	go func() {
		wg.Wait()
		log.Printf("Closing errorChan")
		close(errorChan)
	}()
	var firstNonNilError error
	for errVal := range errorChan {
		log.Printf("errVal: %v", errVal)
		if errVal != nil {
			firstNonNilError = errVal
			cancel()
		}
	}
	if firstNonNilError == nil {
		return ErrKnown{"nil", "nil", "nil", nil}
	}
	return firstNonNilError.(ErrKnown)
}

func Judge(payload *Payload) *Result {
	problemsDir := "/Users/madhavjha/src/github.com/maddyonline/problems"
	cli, err := client.NewEnvClient()
	dieOnErr(err)
	testcases := loadTestCases(problemsDir, payload)
	knwonErr := evaluateAll(cli, payload, testcases, ioutil.Discard, ioutil.Discard)
	log.Printf("Finally, in main: %v", knwonErr)
	result := &Result{}
	switch knwonErr.Type {
	case "nil":
		result.Status = Pass
		result.Details = ""
	case "stdout":
		result.Status = Fail
		result.Details = knwonErr.LongDesc
	case "stderr":
		result.Status = Fail
		result.Details = knwonErr.LongDesc
	}
	log.Printf("Output: %v", result)
	return result
}

func solve(payload *Payload, w io.Writer) error {
	return Solution(payload, w)
}

func Solution(payload *Payload, stdout io.Writer) error {
	problemsDir := "/Users/madhavjha/src/github.com/maddyonline/problems"
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	data, err := ioutil.ReadFile(filepath.Join(problemsDir, payload.Problem.Id, "solution.json"))
	if err != nil {
		return err
	}
	v := &Payload{}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return DockerRun(context.Background(), cli, v, stdout, os.Stderr)
}

func Run(payload *Payload, stdout, stderr io.Writer) *Result {
	cli, err := client.NewEnvClient()
	dieOnErr(err)
	r, w := io.Pipe()
	solveErr := make(chan error)
	go func() {
		solveErr <- solve(payload, w)
	}()
	testcases := []*TestCase{&TestCase{
		Input:      strings.NewReader(payload.Stdin),
		Expected:   r,
		fromDocker: false,
	}}
	knwonErr := evaluateAll(cli, payload, testcases, stdout, ioutil.Discard)
	log.Printf("Finally, in main: %v", knwonErr)
	result := &Result{}
	fmt.Printf("here")
	solveError := <-solveErr
	fmt.Printf("not here")
	if solveError != nil {
		result.Status = Fail
		result.Details = solveError.Error()
		return result
	}
	switch knwonErr.Type {
	case "nil":
		result.Status = Pass
		result.Details = ""
	case "stdout":
		result.Status = Fail
		result.Details = knwonErr.LongDesc
	case "stderr":
		result.Status = Fail
		result.Details = knwonErr.LongDesc
	}
	log.Printf("Output: %v", result)
	return result
}
