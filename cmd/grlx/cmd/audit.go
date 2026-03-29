package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/log"
)

var (
	auditDate       string
	auditAction     string
	auditPubkey     string
	auditLimit      int
	auditFailedOnly bool
)

var cmdAudit = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs for farmer actions",
}

var cmdAuditDates = &cobra.Command{
	Use:   "dates",
	Short: "List available audit log dates",
	Run: func(cmd *cobra.Command, args []string) {
		if client.NatsConn == nil {
			if err := client.ConnectNats(); err != nil {
				log.Fatal(err)
			}
		}

		dates, err := client.ListAuditDates()
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, jsonErr := json.Marshal(dates)
			if jsonErr != nil {
				log.Fatal(jsonErr)
			}
			fmt.Println(string(b))
		default:
			if len(dates) == 0 {
				fmt.Println("No audit logs found.")
				return
			}
			headerColor := color.New(color.FgCyan, color.Bold)
			headerColor.Printf("%-14s  %8s  %10s\n", "DATE", "ENTRIES", "SIZE")
			fmt.Println(strings.Repeat("─", 38))
			for _, d := range dates {
				sizeStr := formatBytes(d.SizeBytes)
				fmt.Printf("%-14s  %8d  %10s\n", d.Date, d.EntryCount, sizeStr)
			}
		}
	},
}

var cmdAuditList = &cobra.Command{
	Use:   "list",
	Short: "Query audit log entries",
	Long: `Query audit log entries with optional filters.

Examples:
  grlx audit list                        # today's entries
  grlx audit list --date 2026-03-12      # specific date
  grlx audit list --action cook          # only cook actions
  grlx audit list --failed               # only failed actions
  grlx audit list --limit 20             # last 20 entries`,
	Run: func(cmd *cobra.Command, args []string) {
		if client.NatsConn == nil {
			if err := client.ConnectNats(); err != nil {
				log.Fatal(err)
			}
		}

		params := audit.QueryParams{
			Date:       auditDate,
			Action:     auditAction,
			Pubkey:     auditPubkey,
			Limit:      auditLimit,
			FailedOnly: auditFailedOnly,
		}

		result, err := client.QueryAudit(params)
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, jsonErr := json.Marshal(result)
			if jsonErr != nil {
				log.Fatal(jsonErr)
			}
			fmt.Println(string(b))
		default:
			if len(result.Entries) == 0 {
				fmt.Printf("No audit entries found for %s.\n", result.Date)
				return
			}

			headerColor := color.New(color.FgCyan, color.Bold)
			headerColor.Printf("Audit log for %s (%d entries, showing %d)\n\n",
				result.Date, result.Total, len(result.Entries))

			headerColor.Printf("%-20s  %-12s  %-8s  %-18s  %s\n",
				"TIMESTAMP", "ACTION", "STATUS", "ROLE", "TARGETS")
			fmt.Println(strings.Repeat("─", 80))

			successColor := color.New(color.FgGreen)
			failColor := color.New(color.FgRed)

			for _, e := range result.Entries {
				ts := e.Timestamp.Format("15:04:05")
				status := "OK"
				statusPrint := successColor.Sprintf("%-8s", status)
				if !e.Success {
					status = "FAIL"
					statusPrint = failColor.Sprintf("%-8s", status)
				}

				targets := ""
				if len(e.Targets) > 0 {
					targets = strings.Join(e.Targets, ", ")
					if len(targets) > 30 {
						targets = targets[:27] + "..."
					}
				}

				roleName := e.RoleName
				if roleName == "" {
					roleName = "-"
				}

				fmt.Printf("%-20s  %-12s  %s  %-18s  %s\n",
					ts, e.Action, statusPrint, roleName, targets)

				if !e.Success && e.Error != "" {
					failColor.Printf("  └─ %s\n", e.Error)
				}
			}
		}
	},
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func init() {
	cmdAuditList.Flags().StringVar(&auditDate, "date", "", "Filter by date (YYYY-MM-DD, default: today)")
	cmdAuditList.Flags().StringVar(&auditAction, "action", "", "Filter by action name (e.g. cook, pki.accept)")
	cmdAuditList.Flags().StringVar(&auditPubkey, "pubkey", "", "Filter by user pubkey")
	cmdAuditList.Flags().IntVar(&auditLimit, "limit", 50, "Maximum entries to return")
	cmdAuditList.Flags().BoolVar(&auditFailedOnly, "failed", false, "Show only failed actions")

	cmdAudit.AddCommand(cmdAuditDates)
	cmdAudit.AddCommand(cmdAuditList)
	rootCmd.AddCommand(cmdAudit)
}
