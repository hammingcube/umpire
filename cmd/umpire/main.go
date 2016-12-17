package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/maddyonline/umpire"
	"os"
	"path/filepath"
)

func main() {
	agent, err := umpire.NewAgent(nil, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	updateCommand := flag.NewFlagSet("update", flag.ExitOnError)
	overwrite := updateCommand.Bool("overwrite", false, "Overwrite existing problems")

	execCommand := flag.NewFlagSet("exec", flag.ExitOnError)
	lang := execCommand.String("lang", "", "programming language (e.g. lang=cpp)")

	validateCommand := flag.NewFlagSet("validate", flag.ExitOnError)

	if len(os.Args) < 2 {
		fmt.Printf("List of subcommands:\n")
		fmt.Printf("update (updates problem cache)\n")
		fmt.Printf("exec   (executes current solution)\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "update":
		updateCommand.Parse(os.Args[2:])
	case "exec":
		execCommand.Parse(os.Args[2:])
	case "validate":
		validateCommand.Parse(os.Args[2:])
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}

	if updateCommand.Parsed() {
		if len(updateCommand.Args()) < 1 {
			fmt.Println("Usage: umpire update <directory>")
			os.Exit(1)
		}
		data, err := umpire.ReadCache()
		if err != nil {
			fmt.Printf("Warning: Got err %v while reading cachefile", err)
			data = map[string]*umpire.JudgeData{}
		}
		if *overwrite {
			fmt.Printf("Overwriting existing cachefile")
			data = map[string]*umpire.JudgeData{}
		}
		for _, input := range updateCommand.Args() {
			dir, err := filepath.Abs(input)
			if err != nil {
				fmt.Printf("Ignoring directory %s because of %v\n", input, err)
				continue
			}
			umpire.ReadAllProblems(data, dir)
		}
		umpire.UpdateCache(data)
		fmt.Printf("updated cache, number of problems: %d\n", len(data))
	}

	if execCommand.Parsed() {
		if len(execCommand.Args()) < 1 {
			fmt.Println("Usage: umpire exec <directory>")
			os.Exit(1)
		}
		var langPriority map[string]int
		if *lang != "" {
			langPriority = map[string]int{*lang: 1}
		}
		payload, err := umpire.ReadSolution(nil, execCommand.Args()[0], langPriority)
		if err != nil {
			fmt.Printf("error: %v", err)
			os.Exit(1)
		}
		exec(agent, payload)
	}

	if validateCommand.Parsed() {
		if len(validateCommand.Args()) < 1 {
			fmt.Println("Usage: umpire validate <directory>")
			os.Exit(1)
		}
		data := map[string]*umpire.JudgeData{}
		for _, input := range validateCommand.Args() {
			dir, err := filepath.Abs(input)
			if err != nil {
				fmt.Printf("Ignoring directory %s because of %v\n", input, err)
				continue
			}
			umpire.ReadAllProblems(data, dir)
		}
		for k, jd := range data {
			json.NewEncoder(os.Stdout).Encode(jd)
			fmt.Println("^^^")
			err, resp := umpire.Validate(agent, jd)
			fmt.Printf("Validating %s, got err=%v and resp=%#v\n", k, err, resp)
		}
	}

}

func exec(agent *umpire.Agent, incoming *umpire.Payload) {
	json.NewEncoder(os.Stdout).Encode(umpire.ExecuteDefault(agent, incoming))
}
