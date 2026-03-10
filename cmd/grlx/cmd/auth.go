package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
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
	authCmd.AddCommand(authWhoAmICmd)
	authCmd.AddCommand(authUsersCmd)
	authCmd.AddCommand(authRolesCmd)
	rootCmd.AddCommand(authCmd)
}

var authWhoAmICmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the identity and role of the current CLI user",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		err := client.CreateSecureTransport()
		if err != nil {
			log.Fatal(err)
		}
		info, err := client.WhoAmI()
		switch outputMode {
		case "json":
			result := struct {
				Pubkey string `json:"pubkey"`
				Role   string `json:"role"`
				Error  string `json:"error,omitempty"`
			}{Pubkey: info.Pubkey, Role: info.RoleName}
			if err != nil {
				result.Error = err.Error()
			}
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Printf("Pubkey: %s\nRole:   %s\n", info.Pubkey, info.RoleName)
		}
	},
}

var authUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "List all configured users and their roles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		err := client.CreateSecureTransport()
		if err != nil {
			log.Fatal(err)
		}
		result, err := client.ListUsers()
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			fmt.Println("Users:")
			for pubkey, roleName := range result.Users {
				fmt.Printf("  %s → %s\n", pubkey, roleName)
			}
			if len(result.Roles) > 0 {
				fmt.Println("\nRoles:")
				for _, role := range result.Roles {
					fmt.Printf("  %s:\n", role.Name)
					for _, rule := range role.Rules {
						if rule.Scope == "" || rule.Scope == "*" {
							fmt.Printf("    - %s (all)\n", rule.Action)
						} else {
							fmt.Printf("    - %s → %s\n", rule.Action, rule.Scope)
						}
					}
				}
			}
		}
	},
}

var authRolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "List all configured role definitions",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		err := client.CreateSecureTransport()
		if err != nil {
			log.Fatal(err)
		}
		result, err := client.ListUsers()
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(result.Roles)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "":
			fallthrough
		case "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			if len(result.Roles) == 0 {
				fmt.Println("No roles configured.")
				return
			}
			for _, role := range result.Roles {
				fmt.Printf("%s:\n", role.Name)
				for _, rule := range role.Rules {
					if rule.Scope == "" || rule.Scope == "*" {
						fmt.Printf("  - %s (all)\n", rule.Action)
					} else {
						fmt.Printf("  - %s → %s\n", rule.Action, rule.Scope)
					}
				}
			}
		}
	},
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
