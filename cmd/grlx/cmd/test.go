package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	"github.com/fatih/color"
	test "github.com/gogrlx/grlx/v2/cmd/grlx/ingredients/test"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
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
	testCmdPing.Flags().BoolVarP(&targetAll, "all", "A", false, "Ping all Sprouts")
	testCmd.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "List of target Sprouts")
	testCmd.AddCommand(testCmdPing)
	rootCmd.AddCommand(testCmd)
}

var testCmdPing = &cobra.Command{
	Use:   "ping [key id]",
	Short: "Determine if a given Sprout is online",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if targetAll {
			sproutTarget = ".*"
		}
		results, err := test.FPing(sproutTarget)
		// TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case pki.ErrSproutIDNotFound:
				log.Fatalf("A targeted Sprout does not exist or is not accepted..")
			default:
				log.Panic(err)
			}
		}
		switch outputMode {
		case "json":
			// TODO: Unmarshall the array specifically instead of the results object
			jw, _ := json.Marshal(results)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			for keyID, result := range results.Results {
				jw, err := json.Marshal(result)
				if err != nil {
					color.Red("%s: \n returned an invalid message!\n", keyID)
					continue
				}
				var value apitypes.PingPong
				err = json.NewDecoder(bytes.NewBuffer(jw)).Decode(&value)
				if err != nil {
					color.Red("%s returned an invalid message!\n", keyID)
				}
				if value.Pong {
					fmt.Printf("%s: \"pong!\"\n", keyID)
				} else {
					color.Red("%s is offline!\n", keyID)
				}
			}
			return
		}
	},
}
