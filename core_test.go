package umpire

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/client"
	"strings"
	"testing"
)

const rawjson = `{
  "problem": {
    "id": ""
  },
  "language": "cpp",
  "stdin": "",
  "files": [
    {
      "name": "main.cpp",
      "content": "# include <iostream>\n\nint main() {\n    std::cout << \"hello\\n\";\n    std::cout << \"hello\";\n}"
    }
  ]
}
`

func Main() {
	cli, err := client.NewEnvClient()
	if err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
	p := &Payload{}
	if err := json.NewDecoder(strings.NewReader(rawjson)).Decode(p); err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
	v, err := payloadRun(context.Background(), cli, p)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Decoded: %#v\n", v)
}

func TestMain(t *testing.T) {
	Main()
}
