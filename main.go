package main

import (
	"bufio"
	_ "encoding/json"
	"github.com/docker/engine-api/client"
	"github.com/maddyonline/umpire/judge"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

const CPP_CODE = `
# include <iostream>
# include <chrono>
# include <thread>


using namespace std;
int main() {
  string s;
  while(cin >> s) {
  	std::this_thread::sleep_for(std::chrono::milliseconds(500));
    cout << s.size() << endl;
  }
}
`

var payloadExample = &judge.Payload{
	Problem:  &judge.Problem{"problem-1"},
	Language: "cpp",
	Files: []*judge.InMemoryFile{
		&judge.InMemoryFile{
			Name:    "main.cpp",
			Content: CPP_CODE,
		},
	},
	Stdin: "",
}

func dieOnErr(err error) {
	if err != nil {
		log.Println(err)
		//os.Exit(1)
	}
}

type TestCase struct {
	Input    io.Reader
	Expected io.Reader
}

type Result struct {
	Status string `json:"status"`
}

func createDirectoryWithFiles(files []*judge.InMemoryFile) (*string, error) {
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

func Evaluate(ctx context.Context, cli *client.Client, payload *judge.Payload, testcase *TestCase) error {
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
	payloadToSend := &judge.Payload{}
	*payloadToSend = *payload
	payloadToSend.Stdin = string(testcaseData)
	result, err := judge.DockerEval(ctx, cli, payloadToSend)
	if err != nil {
		return err
	}
	defer func() {
		if err := result.Cleanup(); err != nil {
			log.Fatal("Error cleaning up container: %v", err)
		}
	}()
	status := make(chan error)
	go func() {
		log.Println("Wating before reading...")
		time.Sleep(5 * time.Second)
		log.Println("Now reading...")
		scanner1 := bufio.NewScanner(result.Reader)
		scanner2 := bufio.NewScanner(testcase.Expected)
		for scanner1.Scan() {
			scanner2.Scan()
			text1, text2 := scanner1.Text(), scanner2.Text()
			text1 = text1[8:] // <---NOTE: SOME DOCKER QUIRK
			log.Printf("output: %s, expected: %s", text1, text2)
			if text1 != text2 {
				log.Printf("->%v<->%v<->%v<-", []byte(text1), []byte(text2), text1 == text2)
				status <- ErrMismatch{}
				return
			}
			for _, scanner := range []*bufio.Scanner{scanner1, scanner2} {
				if err := scanner.Err(); err != nil {
					log.Printf("%v", err)
					status <- err
					return
				}
			}
		}
		status <- nil
	}()
	for {
		select {
		case <-result.Done:
			log.Println("Done!")
		case <-time.After(2 * time.Second):
			log.Println("Still going...")
			// result.cancel()
		case err := <-status:
			if err != nil {
				log.Printf("Now: %v", err)
				//result.cancel()
			}
			return err
		case <-ctx.Done():
			result.Cancel()
			return nil
		}
	}

}

func loadTestCases(problemsDir string, payload *judge.Payload) []*TestCase {
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
	return testcases
}

func EvaluateAll(cli *client.Client, testcases []*TestCase) error {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	errorChan := make(chan error)
	for i, testcase := range testcases {
		wg.Add(1)
		go func(testcase *TestCase) {
			defer func() {
				log.Printf("Done evaluating testcase %d", i)
				wg.Done()
			}()
			err := Evaluate(ctx, cli, payloadExample, testcase)
			if err != nil {
				log.Printf("In main, got %v error", err)
			}
			errorChan <- err
		}(testcase)
	}
	go func() {
		wg.Wait()
		log.Printf("Closing errorChan")
		close(errorChan)
	}()
	var errVal error
	for errVal = range errorChan {
		log.Printf("Err: %v", errVal)
		if errVal != nil && errVal.Error() == "Mismatched" {
			log.Printf("YO: %v", errVal)
			cancel()
		}
	}
	return errVal
}

func main() {
	problemsDir := "/Users/madhavjha/src/github.com/maddyonline/problems"
	cli, err := client.NewEnvClient()
	dieOnErr(err)
	testcases := loadTestCases(problemsDir, payloadExample)
	err = EvaluateAll(cli, testcases)
	log.Printf("Finally, in main: %v", err)
}
