/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	gcook "github.com/gogrlx/grlx/cook/api"
	"github.com/gogrlx/grlx/types"
)

var async bool

// cmdCmd represents the cmd command
var cookCmd = &cobra.Command{
	Use:   "cook",
	Short: "Cook a recipe on a Sprout or a list of Sprouts",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var cmdCook = &cobra.Command{
	Use:   "cook <recipe> <target> [and optional args]...",
	Short: "Cook a recipe against a target and see the output locally.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			cmd.Help()
			return
		}
		var cmdCook types.CmdCook
		results, err := gcook.CookClient(sproutTarget, cmdCook)
		if err != nil {
			switch err {
			case types.ErrSproutIDNotFound:
				log.Fatalf("A targeted Sprout does not exist or is not accepted.")
			default:
				// TODO: handle endpoint timeouts here
				// TODO: Error running command on the Sprout: nats: no responders available for request  run.go:65
				log.Panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(results)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			//			for keyID, result := range results.Results {
			//				jw, err := json.Marshal(result)
			//				if err != nil {
			//					color.Red("%s: \n returned an invalid message!\n", keyID)
			//					continue
			//				}
			//				var value types.CmdRun
			//				err = json.NewDecoder(bytes.NewBuffer(jw)).Decode(&value)
			//				if err != nil {
			//					color.Red("%s returned an invalid message!\n", keyID)
			//					continue
			//				}
			//				if value.ErrCode != 0 {
			//					color.Red("%s:\n", keyID)
			//				} else {
			//					fmt.Printf("%s:\n", keyID)
			//				}
			//				if noerr {
			//					fmt.Printf("%s\n", value.Stdout)
			//				} else {
			//					fmt.Printf("%s%s\n", value.Stdout, value.Stderr)
			//				}
			//			}
		}
	},
}

func init() {
	cmdCook.Flags().StringVarP(&environment, "environment", "E", "", "")
	cmdCook.Flags().BoolVar(&async, "async", false, "Don't print any output, just return the JID to look up results later")
	cmdCook.Flags().StringVarP(&user, "runas", "u", "", "If running as a sudoer, run the command as another user")
	cmdCook.Flags().StringVarP(&cwd, "cwd", "w", "", "Current working directory to run the command in")
	cmdCook.Flags().IntVar(&timeout, "timeout", 30, "Cancel command execution and return after X seconds, printing the JID")
	cmdCook.Flags().StringVarP(&path, "path", "p", "", "Prepend a folder to the PATH before execution")
	cmdCook.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "list of sprouts to target")
	cmdCook.MarkPersistentFlagRequired("target")
	rootCmd.AddCommand(cmdCook)
}
