package umpire

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/maddyonline/umpire/pkg/dockerutils"
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
	cli := dockerutils.NewClient()
	if cli == nil {
		fmt.Printf("Failed to initialize docker client")
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
