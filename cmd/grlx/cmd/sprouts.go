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

var (
	sproutsStateFilter string
	sproutsOnlineOnly  bool
	sproutsCohort      string
)

var cmdSprouts = &cobra.Command{
	Use:   "sprouts",
	Short: "List and inspect connected sprouts",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var cmdSproutsList = &cobra.Command{
	Use:   "list",
	Short: "List all known sprouts with status",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		sprouts, err := client.ListSprouts()
		if err != nil {
			log.Fatalf("Failed to list sprouts: %v", err)
		}

		// If -C/--cohort is given, resolve the cohort and restrict the list
		// to only the sprouts in that cohort.
		var cohortMembers map[string]bool
		if sproutsCohort != "" {
			members, cohortErr := client.ResolveCohort(sproutsCohort)
			if cohortErr != nil {
				log.Fatalf("Failed to resolve cohort %q: %v", sproutsCohort, cohortErr)
			}
			cohortMembers = make(map[string]bool, len(members))
			for _, m := range members {
				cohortMembers[m] = true
			}
		}

		// Apply filters.
		filtered := sprouts[:0]
		for _, s := range sprouts {
			if cohortMembers != nil && !cohortMembers[s.ID] {
				continue
			}
			if sproutsStateFilter != "" && s.KeyState != sproutsStateFilter {
				continue
			}
			if sproutsOnlineOnly && !s.Connected {
				continue
			}
			filtered = append(filtered, s)
		}

		// Sort: connected first, then alphabetical.
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].Connected != filtered[j].Connected {
				return filtered[i].Connected
			}
			return filtered[i].ID < filtered[j].ID
		})

		switch outputMode {
		case "json":
			jw, _ := json.Marshal(filtered)
			fmt.Println(string(jw))
		default:
			if len(filtered) == 0 {
				fmt.Println("No sprouts found.")
				return
			}

			// Calculate column widths.
			maxID := len("SPROUT ID")
			maxState := len("KEY STATE")
			for _, s := range filtered {
				if len(s.ID) > maxID {
					maxID = len(s.ID)
				}
				if len(s.KeyState) > maxState {
					maxState = len(s.KeyState)
				}
			}

			header := fmt.Sprintf("%-*s  %-*s  %s", maxID, "SPROUT ID", maxState, "KEY STATE", "STATUS")
			fmt.Println(header)
			fmt.Println(strings.Repeat("-", len(header)+2))

			for _, s := range filtered {
				status := "offline"
				statusColor := color.RedString
				if s.Connected {
					status = "online"
					statusColor = color.GreenString
				}
				if s.KeyState != "accepted" {
					status = "-"
					statusColor = func(s string, _ ...interface{}) string { return s }
				}

				stateStr := s.KeyState
				switch s.KeyState {
				case "accepted":
					stateStr = color.GreenString(stateStr)
				case "unaccepted":
					stateStr = color.YellowString(stateStr)
				case "denied", "rejected":
					stateStr = color.RedString(stateStr)
				}

				fmt.Printf("%-*s  %-*s  %s\n", maxID, s.ID, maxState, stateStr, statusColor(status))
			}

			// Summary line.
			accepted := 0
			online := 0
			for _, s := range sprouts {
				if s.KeyState == "accepted" {
					accepted++
					if s.Connected {
						online++
					}
				}
			}
			fmt.Printf("\n%d sprout(s) accepted, %d online\n", accepted, online)
		}
	},
}

var cmdSproutsShow = &cobra.Command{
	Use:   "show <sprout-id>",
	Short: "Show detailed info for a sprout",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sproutID := args[0]
		info, err := client.GetSprout(sproutID)
		if err != nil {
			log.Fatalf("Failed to get sprout %q: %v", sproutID, err)
		}

		switch outputMode {
		case "json":
			// Enrich with props if available.
			propsData, propsErr := client.GetSproutProps(sproutID)
			result := map[string]any{
				"sprout": info,
			}
			if propsErr == nil && propsData != nil {
				result["props"] = propsData
			}
			jw, _ := json.Marshal(result)
			fmt.Println(string(jw))
		default:
			fmt.Printf("Sprout:    %s\n", info.ID)
			fmt.Printf("Key State: %s\n", info.KeyState)
			if info.KeyState == "accepted" {
				if info.Connected {
					color.Green("Status:    online")
				} else {
					color.Red("Status:    offline")
				}
			}
			if info.NKey != "" {
				fmt.Printf("NKey:      %s\n", info.NKey)
			}

			// Show props if available.
			propsData, propsErr := client.GetSproutProps(sproutID)
			if propsErr == nil && propsData != nil && len(propsData) > 0 {
				fmt.Println("\nProperties:")
				// Sort keys for stable output.
				keys := make([]string, 0, len(propsData))
				for k := range propsData {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Printf("  %-20s  %v\n", k, propsData[k])
				}
			}
		}
	},
}

func init() {
	cmdSproutsList.Flags().StringVar(&sproutsStateFilter, "state", "", "Filter by key state (accepted, unaccepted, denied, rejected)")
	cmdSproutsList.Flags().BoolVar(&sproutsOnlineOnly, "online", false, "Show only online (connected) sprouts")
	cmdSproutsList.Flags().StringVarP(&sproutsCohort, "cohort", "C", "", "Filter sprouts to members of a cohort")
	cmdSprouts.AddCommand(cmdSproutsList)
	cmdSprouts.AddCommand(cmdSproutsShow)
	rootCmd.AddCommand(cmdSprouts)
}
