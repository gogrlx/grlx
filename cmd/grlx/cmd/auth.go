package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/auth"
)

// testCmd represents the test command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Commands for authentication information",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Help()
	},
}

func init() {
	authCmd.AddCommand(authPrivKeyCmd)
	authCmd.AddCommand(authPubKeyCmd)
	authCmd.AddCommand(authTokenCmd)
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
			status := struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			}{Success: err == nil}
			if err != nil {
				status.Error = err.Error()
			}
			jw, _ := json.Marshal(status)
			fmt.Println(string(jw))
			os.Exit(1)
			return
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Println("Private key saved to config")
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
			}{Pubkey: pubKey}
			if err != nil {
				pubkey.Error = err.Error()
			}
			jw, _ := json.Marshal(pubkey)
			// TODO: Unmarshall the array specifically instead of the results object
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
			return
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Println(pubKey)
		}
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Create token for the Authorization header of API requests",
	Long:  `Token is valid for 5 minutes`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		token, err := auth.NewToken()
		switch outputMode {
		case "json":
			token := struct {
				Token string `json:"token"`
				Error string `json:"error"`
			}{Token: token}
			if err != nil {
				token.Error = err.Error()
			}
			jw, _ := json.Marshal(token)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
			return
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Println(token)
		}
	},
}
