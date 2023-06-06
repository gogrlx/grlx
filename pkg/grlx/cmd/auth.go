/*
Copyright © 2021 Tai Groot <tai@taigrr.com>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/auth"
)

// testCmd represents the test command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "commands for authentication information",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Help()
	},
}

func init() {
	authCmd.AddCommand(authPubKeyCmd)
	authCmd.AddCommand(authPrivKeyCmd)
	rootCmd.AddCommand(authCmd)
}

var authPrivKeyCmd = &cobra.Command{
	Use:   "privkey",
	Short: "Create a private key for the grlx CLI",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		err := auth.CreatePrivkey()

		switch outputMode {
		case "json":
			success := "true"
			if err != nil {
				success = "false"
			}
			status := struct {
				Success string `json:"success"`
				Error   string `json:"error"`
			}{Success: success, Error: err.Error()}
			jw, _ := json.Marshal(status)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				return
			}
			fmt.Println("Private key saved to config")
		case "yaml":
			// TODO implement YAML
		}
	},
}

var authPubKeyCmd = &cobra.Command{
	Use:   "pubkey",
	Short: "Get the public key of the grlx CLI",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		pubKey, err := auth.GetPubkey()
		switch outputMode {
		case "json":
			pubkey := struct {
				Pubkey string `json:"pubkey"`
				Error  string `json:"error"`
			}{Pubkey: pubKey, Error: err.Error()}
			jw, _ := json.Marshal(pubkey)
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
