/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/api/client"
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
		results, err := client.Cook(sproutTarget, cmdCook)
		if err != nil {
			switch err {
			case types.ErrSproutIDNotFound:
				log.Fatalf("A targeted Sprout does not exist or is not accepted.")
			default:
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
	cmdCook.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "list of sprouts to target")
	cmdCook.MarkPersistentFlagRequired("target")
	rootCmd.AddCommand(cmdCook)
}
