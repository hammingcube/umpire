// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/maddyonline/umpire"
	"github.com/maddyonline/umpire/pkg/dockerutils"
	"github.com/spf13/cobra"
	"log"
	"path/filepath"
)

func validate(args []string) {
	agent := &umpire.Agent{
		Client: dockerutils.NewClient(),
		Data:   make(map[string]*umpire.JudgeData),
	}
	data := map[string]*umpire.JudgeData{}
	for _, input := range args {
		dir, err := filepath.Abs(input)
		if err != nil {
			log.Printf("Ignoring directory %s because of %v", input, err)
			continue
		}
		umpire.ReadAllProblems(data, dir)
	}
	for k, jd := range data {
		err, resp := umpire.Validate(agent, jd)
		log.Printf("Validating %s, got err=%v and resp=%#v", k, err, resp)
	}
}

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validates a problem directory",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: umpire validate <directory>")
			return
		}
		validate(args)
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// validateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// validateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
