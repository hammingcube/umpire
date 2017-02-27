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
	"github.com/maddyonline/umpire"
	"github.com/spf13/cobra"
	"log"
	"path/filepath"
)

var (
	overwrite bool
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Println("Usage: umpire update <directory>")
			return
		}
		data, err := umpire.ReadCache()
		if err != nil {
			log.Printf("Warning: Got err %v while reading cachefile", err)
			data = map[string]*umpire.JudgeData{}
		}
		if overwrite {
			log.Printf("Overwriting existing cachefile")
			data = map[string]*umpire.JudgeData{}
		}
		for _, input := range args {
			dir, err := filepath.Abs(input)
			if err != nil {
				log.Printf("Ignoring directory %s because of %v\n", input, err)
				continue
			}
			umpire.ReadAllProblems(data, dir)
		}
		umpire.UpdateCache(data)
		log.Printf("updated cache, number of problems: %d\n", len(data))
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// updateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	updateCmd.Flags().BoolVarP(&overwrite, "overwrite", "w", false, "Overwrite cache file")

}
