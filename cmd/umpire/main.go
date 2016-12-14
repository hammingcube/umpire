package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/maddyonline/umpire"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	updateCommand := flag.NewFlagSet("update", flag.ExitOnError)
	overwrite := updateCommand.Bool("overwrite", false, "Overwrite existing problems")

	if len(os.Args) < 2 {
		fmt.Println("update is required")
		fmt.Println("update")
		updateCommand.PrintDefaults()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "update":
		updateCommand.Parse(os.Args[2:])
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}

	if updateCommand.Parsed() {
		if len(updateCommand.Args()) < 1 {
			fmt.Println("Usage: umpire update <directory>")
			os.Exit(1)
		}
		if *overwrite {
			fmt.Println("overwriting")
		}
		fmt.Printf("Got following args: %#v\n", updateCommand.Args())
		dir, _ := filepath.Abs(updateCommand.Args()[0])
		data := map[string]*umpire.JudgeData{}
		umpire.ReadAllProblems(data, dir)
		umpire.UpdateCache(data)
		fmt.Printf("data: %#v\n", data)
	}

}

var rawpayload = `{"problem":{"id":"prob-2"},"language":"cpp","stdin":"here\nhellotherehowareyou\ncol\nteh\reallynice\ncurse\nof\ndimensionality\n","files":[{"name":"main.cpp","content":"# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"}]}`

func runMain() {
	agent, err := umpire.NewAgent(nil, nil)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
	incoming := &umpire.Payload{}
	if err := json.NewDecoder(strings.NewReader(rawpayload)).Decode(incoming); err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
	json.NewEncoder(os.Stdout).Encode(umpire.ExecuteDefault(agent, incoming))
}
