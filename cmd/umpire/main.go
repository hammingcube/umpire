package main

import (
	"encoding/json"
	"fmt"
	"github.com/maddyonline/umpire"
	"os"
	"strings"
)

var rawpayload = `{"problem":{"id":"prob-2"},"language":"cpp","stdin":"here\nhellotherehowareyou\ncol\nteh\reallynice\ncurse\nof\ndimensionality\n","files":[{"name":"main.cpp","content":"# include <iostream>\nusing namespace std;\nint main() {string s;while(cin >> s) {cout << s.size() << endl;}}"}]}`

func main() {
	agent, err := umpire.NewAgent(nil, nil, nil)
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
