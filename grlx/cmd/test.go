/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	test "github.com/gogrlx/grlx/grlx/ingredients/test"
	. "github.com/gogrlx/grlx/types"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Various utilities to monitor and test Sprout connections",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	testCmd_Ping.Flags().BoolVarP(&targetAll, "all", "A", false, "Ping all Sprouts")
	testCmd.AddCommand(testCmd_Ping)
	rootCmd.AddCommand(testCmd)

}

var testCmd_Ping = &cobra.Command{
	Use:   "ping [key id]",
	Short: "Determine if a given Sprout is online",
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		ok, err := test.FPing(keyID)
		//TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case ErrSproutIDNotFound:
				log.Fatalf("Sprout %s does not exist.", keyID)
			case ErrAlreadyAccepted:
				log.Fatalf("Sprout %s has already been accepted.", keyID)
			default:
				panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(ok)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if ok {
				fmt.Printf("%s: \"pong!\"\n", keyID)
				return
			}
			color.Red("%s is offline!\n", keyID)
			os.Exit(1)
		case "yaml":
			//TODO implement YAML
		}

	},
}
