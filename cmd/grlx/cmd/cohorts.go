package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/log"
)

var cmdCohorts = &cobra.Command{
	Use:   "cohorts",
	Short: "Manage and inspect sprout cohorts",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var cmdCohortsList = &cobra.Command{
	Use:   "list",
	Short: "List all configured cohorts",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := client.NatsRequest("cohorts.list", nil)
		if err != nil {
			log.Fatalf("Failed to list cohorts: %v", err)
		}

		var result struct {
			Cohorts []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"cohorts"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			log.Fatalf("Failed to decode response: %v", err)
		}

		switch outputMode {
		case "json":
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
		default:
			if len(result.Cohorts) == 0 {
				fmt.Println("No cohorts configured.")
				return
			}
			for _, c := range result.Cohorts {
				fmt.Printf("%-20s  [%s]\n", c.Name, c.Type)
			}
		}
	},
}

var cmdCohortsShow = &cobra.Command{
	Use:   "show <name>",
	Short: "Show the full definition of a cohort",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		detail, err := client.GetCohort(name)
		if err != nil {
			color.Red("Error: %v", err)
			return
		}

		switch outputMode {
		case "json":
			jw, _ := json.MarshalIndent(detail, "", "  ")
			fmt.Println(string(jw))
		default:
			fmt.Printf("Cohort: %s\n", detail.Name)
			fmt.Printf("  Type: %s\n", detail.Type)

			switch detail.Type {
			case "static":
				fmt.Printf("  Configured members: %d\n", len(detail.Members))
				for _, m := range detail.Members {
					fmt.Printf("    - %s\n", m)
				}
			case "dynamic":
				if detail.Match != nil {
					fmt.Printf("  Match: %s = %s\n", detail.Match.PropName, detail.Match.PropValue)
				}
			case "compound":
				if detail.Compound != nil {
					fmt.Printf("  Operator: %s\n", detail.Compound.Operator)
					fmt.Printf("  Operands:\n")
					for _, op := range detail.Compound.Operands {
						fmt.Printf("    - %s\n", op)
					}
				}
			}

			fmt.Printf("  Resolved members: %d\n", detail.Count)
			for _, id := range detail.Resolved {
				fmt.Printf("    - %s\n", id)
			}
		}
	},
}

var cmdCohortsResolve = &cobra.Command{
	Use:   "resolve <name>",
	Short: "Resolve a cohort to its member sprout IDs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		sprouts, err := client.ResolveCohort(name)
		if err != nil {
			color.Red("Error: %v", err)
			return
		}

		switch outputMode {
		case "json":
			result := map[string]interface{}{
				"name":    name,
				"sprouts": sprouts,
			}
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
		default:
			fmt.Printf("Cohort %q (%d sprout(s)):\n", name, len(sprouts))
			for _, id := range sprouts {
				fmt.Printf("  %s\n", id)
			}
		}
	},
}

func init() {
	cmdCohorts.AddCommand(cmdCohortsList)
	cmdCohorts.AddCommand(cmdCohortsShow)
	cmdCohorts.AddCommand(cmdCohortsResolve)
	rootCmd.AddCommand(cmdCohorts)
}
