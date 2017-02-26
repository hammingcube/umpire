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
	"encoding/json"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/maddyonline/umpire"
	"github.com/maddyonline/umpire/pkg/dockerutils"
	"github.com/spf13/cobra"
	"os"
)

func NewClient() *client.Client {
	dockerutils.InitMachines([]string{"local", "myremotedocker"})
	if m := dockerutils.GetMachine(); m != nil {
		return m
	}
	return nil
}

var language string

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "compiles and runs files in specified directory",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("exec called with args: %v with language=%s\n", args, language)
		dockerClient := NewClient()
		if dockerClient == nil {
			return
		}
		agent, err := umpire.NewAgent(dockerClient, nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		stdin := ""
		if len(args) > 1 {
			stdin = args[1]
		}
		payload := &umpire.Payload{}
		if _, err := umpire.LoadFiles(payload, args[0], language, stdin); err != nil {
			fmt.Printf("Err: %v\n", err)
			return
		}
		json.NewEncoder(os.Stdout).Encode(payload)
		exec(agent, payload)
	},
}

func exec(agent *umpire.Agent, incoming *umpire.Payload) {
	json.NewEncoder(os.Stdout).Encode(umpire.ExecuteDefault(agent, incoming))
}

func init() {
	RootCmd.AddCommand(execCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// execCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	execCmd.Flags().StringVarP(&language, "lang", "L", "cpp", "Programming language of source file(s)")

}
