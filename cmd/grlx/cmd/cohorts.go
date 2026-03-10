package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/internal/api"
	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
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
		httpClient := client.APIClient
		ctx := context.Background()

		url := config.FarmerURL + api.Routes["ListCohorts"].Pattern
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Accept", "application/json")
		newToken, err := auth.NewToken()
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Authorization", newToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Fatalf("Failed to contact farmer: %v", err)
		}
		defer resp.Body.Close()

		var result struct {
			Cohorts []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"cohorts"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
	cmdCohorts.AddCommand(cmdCohortsResolve)
	rootCmd.AddCommand(cmdCohorts)
}
