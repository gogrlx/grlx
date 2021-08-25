/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var environment string
var user string
var cwd string
var timeout int
var path string

// cmdCmd represents the cmd command
var cmdCmd = &cobra.Command{
	Use:   "cmd",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cmd called")
	},
}
var cmdCmd_Run = &cobra.Command{
	Use:   "run ['command']",
	Short: "Run a command remotely and see the output locally.",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run called")
	},
}

func init() {

	cmdCmd_Run.Flags().StringVarP(&environment, "environment", "E", "", "List of space-separated key=value OS Environment Variables")
	cmdCmd_Run.Flags().StringVarP(&user, "runas", "u", "", "If running as a sudoer, run the command as another user")
	cmdCmd_Run.Flags().StringVarP(&cwd, "cwd", "w", "", "Current working directory to run the command in")
	cmdCmd_Run.Flags().IntVar(&timeout, "timout", 30, "Cancel command execution and return after X seconds")
	cmdCmd_Run.Flags().StringVarP(&path, "path", "p", "", "Prepend a folder to the PATH before execution")
	cmdCmd.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "list of sprouts to target")
	cmdCmd.MarkPersistentFlagRequired("target")
	cmdCmd.AddCommand(cmdCmd_Run)
	rootCmd.AddCommand(cmdCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// cmdCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cmdCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
