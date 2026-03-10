package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/gogrlx/grlx/v2/internal/log"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

var (
	jobsLimit    int
	watchTimeout int
)

var cmdJobs = &cobra.Command{
	Use:   "jobs",
	Short: "Manage and inspect cook jobs",
}

var cmdJobsList = &cobra.Command{
	Use:   "list [sproutID]",
	Short: "List recent jobs, optionally filtered by sprout",
	Run: func(cmd *cobra.Command, args []string) {
		var summaries []jobs.JobSummary
		var err error

		if len(args) > 0 {
			summaries, err = client.ListJobsForSprout(args[0])
		} else {
			summaries, err = client.ListJobs(jobsLimit)
		}
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, err := json.Marshal(summaries)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(b))
		default:
			if len(summaries) == 0 {
				fmt.Println("No jobs found.")
				return
			}
			printJobsTable(summaries)
		}
	},
}

var cmdJobsShow = &cobra.Command{
	Use:   "show <JID>",
	Short: "Show details of a specific job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jid := args[0]
		summary, err := client.GetJob(jid)
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, err := json.Marshal(summary)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(b))
		default:
			printJobDetail(summary)
		}
	},
}

var cmdJobsWatch = &cobra.Command{
	Use:   "watch <JID>",
	Short: "Watch a running job's progress in real time",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jid := args[0]

		nc, err := client.NewNatsClient()
		if err != nil {
			log.Fatal(err)
		}
		defer nc.Flush()

		topic := fmt.Sprintf("grlx.cook.*.%s", jid)
		finished := make(chan struct{}, 1)

		sub, err := nc.Subscribe(topic, func(msg *nats.Msg) {
			var step cook.StepCompletion
			if err := json.Unmarshal(msg.Data, &step); err != nil {
				log.Errorf("Error unmarshalling message: %v\n", err)
				return
			}

			subComponents := strings.Split(msg.Subject, ".")
			sproutID := subComponents[2]

			if strings.HasPrefix(string(step.ID), "completed-") {
				printTex.Lock()
				fmt.Printf("\n%s :: Job %s completed on %s\n",
					color.GreenString("DONE"), jid, sproutID)
				printTex.Unlock()
				finished <- struct{}{}
				return
			}
			if strings.HasPrefix(string(step.ID), "start-") {
				printTex.Lock()
				fmt.Printf("%s :: Job %s started on %s\n",
					color.CyanString("START"), jid, sproutID)
				printTex.Unlock()
				return
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s::%s\n", sproutID, jid))
			b.WriteString(fmt.Sprintf("  ID: %s\n", step.ID))
			switch step.CompletionStatus {
			case cook.StepCompleted:
				b.WriteString(fmt.Sprintf("  Result: %s\n", color.GreenString("Success")))
			case cook.StepFailed:
				b.WriteString(fmt.Sprintf("  Result: %s\n", color.RedString("Failure")))
			case cook.StepSkipped:
				b.WriteString(fmt.Sprintf("  Result: %s\n", color.YellowString("Skipped")))
			default:
				b.WriteString(fmt.Sprintf("  Result: %s\n", color.YellowString("Unknown")))
			}
			for _, change := range step.Changes {
				b.WriteString(fmt.Sprintf("    %s\n", change))
			}
			if !step.Started.IsZero() {
				b.WriteString(fmt.Sprintf("  Duration: %s\n", step.Duration.Round(time.Millisecond)))
			}

			printTex.Lock()
			fmt.Print(b.String())
			printTex.Unlock()
		})
		if err != nil {
			log.Fatal(err)
		}
		defer sub.Unsubscribe()

		fmt.Printf("Watching job %s (timeout %ds)...\n", jid, watchTimeout)

		select {
		case <-finished:
		case <-time.After(time.Duration(watchTimeout) * time.Second):
			color.Red("Watch timed out after %d seconds.", watchTimeout)
		}
	},
}

var cmdJobsCancel = &cobra.Command{
	Use:   "cancel <JID>",
	Short: "Cancel a running or pending job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jid := args[0]
		err := client.CancelJob(jid)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Cancel request sent for job %s\n", jid)
	},
}

func printJobsTable(summaries []jobs.JobSummary) {
	// Header
	fmt.Printf("%-20s  %-24s  %-10s  %5s  %5s  %5s  %s\n",
		"JID", "SPROUT", "STATUS", "OK", "FAIL", "SKIP", "STARTED")
	fmt.Println(strings.Repeat("-", 100))

	for _, s := range summaries {
		statusStr := formatStatus(s.Status)
		started := "—"
		if !s.StartedAt.IsZero() {
			started = s.StartedAt.Format(time.RFC3339)
		}
		fmt.Printf("%-20s  %-24s  %-10s  %5d  %5d  %5d  %s\n",
			truncate(s.JID, 20),
			truncate(s.SproutID, 24),
			statusStr,
			s.Succeeded,
			s.Failed,
			s.Skipped,
			started,
		)
	}
}

func printJobDetail(s *jobs.JobSummary) {
	fmt.Printf("Job:     %s\n", s.JID)
	fmt.Printf("Sprout:  %s\n", s.SproutID)
	fmt.Printf("Status:  %s\n", formatStatus(s.Status))
	if !s.StartedAt.IsZero() {
		fmt.Printf("Started: %s\n", s.StartedAt.Format(time.RFC3339))
	}
	if s.Duration > 0 {
		fmt.Printf("Duration: %s\n", s.Duration.Round(time.Millisecond))
	}
	fmt.Printf("Steps:   %d total (%d succeeded, %d failed, %d skipped)\n",
		s.Total, s.Succeeded, s.Failed, s.Skipped)
	fmt.Println()

	if len(s.Steps) == 0 {
		fmt.Println("No steps recorded.")
		return
	}

	fmt.Println("Steps:")
	fmt.Println(strings.Repeat("-", 80))
	for _, step := range s.Steps {
		// Skip the synthetic start/completed markers
		if strings.HasPrefix(string(step.ID), "start-") ||
			strings.HasPrefix(string(step.ID), "completed-") {
			continue
		}

		fmt.Printf("  ID: %s\n", step.ID)
		switch step.CompletionStatus {
		case cook.StepCompleted:
			fmt.Printf("  Result: %s\n", color.GreenString("Success"))
		case cook.StepFailed:
			fmt.Printf("  Result: %s\n", color.RedString("Failure"))
		case cook.StepSkipped:
			fmt.Printf("  Result: %s\n", color.YellowString("Skipped"))
		case cook.StepInProgress:
			fmt.Printf("  Result: %s\n", color.CyanString("In Progress"))
		case cook.StepNotStarted:
			fmt.Printf("  Result: %s\n", "Not Started")
		}
		if len(step.Changes) > 0 {
			fmt.Println("  Notes:")
			for _, change := range step.Changes {
				fmt.Printf("    %s\n", change)
			}
		}
		if !step.Started.IsZero() {
			fmt.Printf("  Started:  %s\n", step.Started.Format(time.RFC3339))
			fmt.Printf("  Duration: %s\n", step.Duration.Round(time.Millisecond))
		}
		fmt.Println()
	}
}

func formatStatus(status jobs.JobStatus) string {
	switch status {
	case jobs.JobSucceeded:
		return color.GreenString("succeeded")
	case jobs.JobFailed:
		return color.RedString("failed")
	case jobs.JobRunning:
		return color.CyanString("running")
	case jobs.JobPending:
		return color.YellowString("pending")
	case jobs.JobPartial:
		return color.YellowString("partial")
	default:
		return "unknown"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func init() {
	cmdJobsList.Flags().IntVar(&jobsLimit, "limit", 50, "Maximum number of jobs to return")
	cmdJobsWatch.Flags().IntVar(&watchTimeout, "timeout", 120, "Watch timeout in seconds")
	cmdJobs.AddCommand(cmdJobsList)
	cmdJobs.AddCommand(cmdJobsShow)
	cmdJobs.AddCommand(cmdJobsWatch)
	cmdJobs.AddCommand(cmdJobsCancel)
	rootCmd.AddCommand(cmdJobs)
}
