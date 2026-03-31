package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/log"
)

var cmdProps = &cobra.Command{
	Use:   "props",
	Short: "View and manage sprout properties",
	Long: `View, set, and delete properties for individual sprouts.

Properties are key-value pairs attached to a sprout that can be used
in recipe templates, cohort matching, and general configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var cmdPropsGet = &cobra.Command{
	Use:   "get <sprout-id> [key]",
	Short: "Get a sprout's properties",
	Long: `Retrieve all properties for a sprout, or a single property by key.

If a key is provided, only that property's value is returned.
Otherwise, all properties are listed.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		sproutID := args[0]

		if len(args) == 2 {
			// Single property lookup.
			key := args[1]
			params := map[string]string{"sprout_id": sproutID, "name": key}
			resp, err := client.NatsRequest("props.get", params)
			if err != nil {
				log.Fatalf("Failed to get property: %v", err)
			}

			var result struct {
				SproutID string `json:"sprout_id"`
				Name     string `json:"name"`
				Value    string `json:"value"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				log.Fatalf("Failed to decode response: %v", err)
			}

			switch outputMode {
			case "json":
				jw, _ := json.Marshal(result)
				fmt.Println(string(jw))
			default:
				fmt.Println(result.Value)
			}
			return
		}

		// All properties.
		propsData, err := client.GetSproutProps(sproutID)
		if err != nil {
			log.Fatalf("Failed to get properties: %v", err)
		}

		switch outputMode {
		case "json":
			jw, _ := json.MarshalIndent(propsData, "", "  ")
			fmt.Println(string(jw))
		default:
			if len(propsData) == 0 {
				fmt.Printf("No properties set for %s.\n", sproutID)
				return
			}
			keys := make([]string, 0, len(propsData))
			for k := range propsData {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%-30s  %v\n", k, propsData[k])
			}
		}
	},
}

var cmdPropsSet = &cobra.Command{
	Use:   "set <sprout-id> <key> <value>",
	Short: "Set a property on a sprout",
	Long: `Set or update a single property on a sprout.

The value is stored as a string. To set structured values, use JSON format.`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		sproutID := args[0]
		key := args[1]
		value := args[2]

		params := map[string]string{
			"sprout_id": sproutID,
			"name":      key,
			"value":     value,
		}
		_, err := client.NatsRequest("props.set", params)
		if err != nil {
			log.Fatalf("Failed to set property: %v", err)
		}

		switch outputMode {
		case "json":
			result := map[string]string{
				"sprout_id": sproutID,
				"name":      key,
				"value":     value,
				"status":    "ok",
			}
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
		default:
			color.Green("Set %s = %s on %s", key, value, sproutID)
		}
	},
}

var cmdPropsDelete = &cobra.Command{
	Use:     "delete <sprout-id> <key>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a property from a sprout",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sproutID := args[0]
		key := args[1]

		params := map[string]string{
			"sprout_id": sproutID,
			"name":      key,
		}
		_, err := client.NatsRequest("props.delete", params)
		if err != nil {
			log.Fatalf("Failed to delete property: %v", err)
		}

		switch outputMode {
		case "json":
			result := map[string]string{
				"sprout_id": sproutID,
				"name":      key,
				"status":    "deleted",
			}
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
		default:
			color.Green("Deleted %s from %s", key, sproutID)
		}
	},
}

var cmdPropsSearch = &cobra.Command{
	Use:   "search <key> [value]",
	Short: "Search for sprouts with a matching property",
	Long: `Search across all accepted sprouts for those that have a given property key.
Optionally filter by value.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		searchKey := args[0]
		var searchValue string
		if len(args) == 2 {
			searchValue = args[1]
		}

		// Get list of sprouts.
		sproutsResp, err := client.NatsRequest("sprouts.list", nil)
		if err != nil {
			log.Fatalf("Failed to list sprouts: %v", err)
		}

		var sproutsList struct {
			Sprouts []struct {
				ID string `json:"id"`
			} `json:"sprouts"`
		}
		if err := json.Unmarshal(sproutsResp, &sproutsList); err != nil {
			log.Fatalf("Failed to decode sprouts: %v", err)
		}

		type match struct {
			SproutID string `json:"sprout_id"`
			Value    string `json:"value"`
		}
		var matches []match

		for _, sp := range sproutsList.Sprouts {
			params := map[string]string{"sprout_id": sp.ID, "name": searchKey}
			resp, err := client.NatsRequest("props.get", params)
			if err != nil {
				continue
			}

			var result struct {
				Value string `json:"value"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				continue
			}

			if result.Value == "" {
				continue
			}

			if searchValue != "" && !strings.Contains(result.Value, searchValue) {
				continue
			}

			matches = append(matches, match{SproutID: sp.ID, Value: result.Value})
		}

		switch outputMode {
		case "json":
			jw, _ := json.Marshal(matches)
			fmt.Println(string(jw))
		default:
			if len(matches) == 0 {
				fmt.Printf("No sprouts found with property %q", searchKey)
				if searchValue != "" {
					fmt.Printf(" containing %q", searchValue)
				}
				fmt.Println(".")
				return
			}
			fmt.Printf("Sprouts with %s", searchKey)
			if searchValue != "" {
				fmt.Printf(" containing %q", searchValue)
			}
			fmt.Printf(" (%d):\n", len(matches))
			for _, m := range matches {
				fmt.Printf("  %-30s  %s\n", m.SproutID, m.Value)
			}
		}
	},
}

func init() {
	cmdProps.AddCommand(cmdPropsGet)
	cmdProps.AddCommand(cmdPropsSet)
	cmdProps.AddCommand(cmdPropsDelete)
	cmdProps.AddCommand(cmdPropsSearch)
	rootCmd.AddCommand(cmdProps)
}
