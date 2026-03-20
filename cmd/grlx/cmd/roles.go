package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// builtinRoleNames lists the names of built-in roles for display purposes.
var builtinRoleNames = map[string]bool{
	"viewer":   true,
	"operator": true,
}

var rolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "List all available roles and their permissions",
	Long: `Lists all RBAC roles defined on the farmer, including built-in roles
(viewer, operator) and any custom roles from the config.

Built-in roles:
  viewer    — read-only access (view sprouts, jobs, props, cohorts)
  operator  — operational access (view + cook, cmd, shell, props, jobs)
  admin     — full access (defined per-config, not built-in)

Custom roles can be defined in the farmer config under the "roles" section.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		result, err := client.ListUsers()
		switch outputMode {
		case "json":
			type roleEntry struct {
				Name    string      `json:"name"`
				Rules   []rbac.Rule `json:"rules"`
				Builtin bool        `json:"builtin"`
			}
			entries := make([]roleEntry, 0, len(result.Roles))
			for _, role := range result.Roles {
				entries = append(entries, roleEntry{
					Name:    role.Name,
					Rules:   role.Rules,
					Builtin: builtinRoleNames[role.Name],
				})
			}
			jw, _ := json.Marshal(entries)
			fmt.Println(string(jw))
			if err != nil {
				os.Exit(1)
			}
		case "", "text":
			if err != nil {
				log.Println("Error: " + err.Error())
				os.Exit(1)
			}
			if len(result.Roles) == 0 {
				fmt.Println("No roles configured.")
				return
			}
			for i, role := range result.Roles {
				suffix := ""
				if builtinRoleNames[role.Name] {
					suffix = " (built-in)"
				}
				fmt.Printf("%s%s\n", role.Name, suffix)
				for _, rule := range role.Rules {
					if rule.Scope == "" || rule.Scope == "*" {
						fmt.Printf("  - %s (all)\n", rule.Action)
					} else {
						fmt.Printf("  - %s → %s\n", rule.Action, rule.Scope)
					}
				}
				if i < len(result.Roles)-1 {
					fmt.Println()
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(rolesCmd)
}
