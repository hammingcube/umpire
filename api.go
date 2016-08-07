package main

import (
	"io"
	"log"
	"os"
)

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

func Evaluate(payload *Payload, testcase io.Reader) error {
	srcDir, language := "/Users/madhavjha/src/github.com/maddyonline/moredocker", "cpp"
	reader, err := DockerEval(srcDir, language, testcase)
	if err != nil {
		return err
	}
	go func() {
		_, err = io.Copy(os.Stdout, reader)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
	}()

	ch := make(chan int)
	<-ch
	return nil
}

func main() {
	testcase, err := os.Open("input-new.txt")
	dieOnErr(err)
	Evaluate(&Payload{}, testcase)
}
