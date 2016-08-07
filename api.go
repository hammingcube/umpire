package main

import (
	"github.com/docker/engine-api/client"
	"io"
	"log"
	"os"
	"time"
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

func Evaluate(cli *client.Client, payload *Payload, testcase io.Reader) error {
	srcDir, language := "/Users/madhavjha/src/github.com/maddyonline/moredocker", "cpp"
	result, err := DockerEval(cli, srcDir, language, testcase)
	defer result.cancel()
	if err != nil {
		return err
	}
	go func() {
		_, err = io.Copy(os.Stdout, result.reader)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
	}()

	for {
		select {
		case <-result.done:
			log.Println("Done! Now removing container...")
			if err := result.cleanup(); err != nil {
				log.Fatal("Error cleaning up container: %v", err)
				return err
			}
			return nil
		case <-time.After(2 * time.Second):
			log.Println("Still going...")
		}
	}

	return nil
}

func main() {
	cli, err := client.NewEnvClient()
	dieOnErr(err)
	testcase, err := os.Open("input-new.txt")
	dieOnErr(err)
	Evaluate(cli, &Payload{}, testcase)
}
