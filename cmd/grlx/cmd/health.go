package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the farmer's health status",
	Run: func(cmd *cobra.Command, _ []string) {
		resp, err := client.Health()
		if err != nil {
			log.Fatalf("Error checking farmer health: %v", err)
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(resp)
			fmt.Println(string(jw))
		case "":
			fallthrough
		case "text":
			fmt.Printf("Status:     %s\n", resp.Status)
			fmt.Printf("Uptime:     %s\n", resp.Uptime)
			fmt.Printf("NATS Ready: %v\n", resp.NATSReady)
		}
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
