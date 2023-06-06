/*
Copyright © 2021 Tai Groot <tai@taigrr.com>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gogrlx/grlx/auth"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "print authentication information to stdout",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Help()
	},
}

func init() {
	authCmd.AddCommand(authPubKeyCmd)
	rootCmd.AddCommand(authCmd)
}

var authPubKeyCmd = &cobra.Command{
	Use:   "pubkey",
	Short: "Get the public key of the grlx CLI",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		pubKey, err := auth.GetPubkey()
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(`{"pubkey": "` + pubKey + `", "error": "` + err.Error() + `"}`)
			// TODO: Unmarshall the array specifically instead of the results object
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				return
			}
			fmt.Println(pubKey)
		case "yaml":
			// TODO implement YAML
		}
	},
}
