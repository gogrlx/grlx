package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage grlx users",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Help()
	},
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured users and their roles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		result, err := client.ListUsers()
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "", "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			if len(result.Users) == 0 {
				fmt.Println("No users configured.")
				return
			}
			fmt.Println("Users:")
			for pubkey, roleName := range result.Users {
				fmt.Printf("  %s → %s\n", pubkey, roleName)
			}
		}
	},
}

var usersAddCmd = &cobra.Command{
	Use:   "add <role> <pubkey>",
	Short: "Add a user with the given role and public key",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		roleName := args[0]
		pubkey := args[1]
		result, err := client.AddUser(pubkey, roleName)
		switch outputMode {
		case "json":
			resp := struct {
				Success bool   `json:"success"`
				Message string `json:"message,omitempty"`
				Error   string `json:"error,omitempty"`
			}{Success: result.Success, Message: result.Message}
			if err != nil {
				resp.Error = err.Error()
			}
			jw, _ := json.Marshal(resp)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "", "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Println(result.Message)
		}
	},
}

var usersRemoveCmd = &cobra.Command{
	Use:   "remove <pubkey>",
	Short: "Remove a user by public key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pubkey := args[0]
		result, err := client.RemoveUser(pubkey)
		switch outputMode {
		case "json":
			resp := struct {
				Success bool   `json:"success"`
				Message string `json:"message,omitempty"`
				Error   string `json:"error,omitempty"`
			}{Success: result.Success, Message: result.Message}
			if err != nil {
				resp.Error = err.Error()
			}
			jw, _ := json.Marshal(resp)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "", "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Println(result.Message)
		}
	},
}

func init() {
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersAddCmd)
	usersCmd.AddCommand(usersRemoveCmd)
	rootCmd.AddCommand(usersCmd)
}
