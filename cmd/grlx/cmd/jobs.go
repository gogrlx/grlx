package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

var (
	jobsLimit    int
	jobsLocal    bool
	jobsUser     string
	jobsCohort   string
	watchTimeout int
	purgeOlderH  int
)

var cmdJobs = &cobra.Command{
	Use:   "jobs",
	Short: "Manage and inspect cook jobs",
}

var cmdJobsList = &cobra.Command{
	Use:   "list [sproutID]",
	Short: "List recent jobs, optionally filtered by sprout",
	Long: `List recent jobs. By default queries the farmer.
Use --local to list jobs from local CLI-side storage.
Use --user to filter jobs by the invoking user's pubkey (use 'me' for current user).
Use -C/--cohort to filter jobs to sprouts in a cohort.
When using --local, an optional sproutID argument filters jobs for that sprout.`,
	Run: func(cmd *cobra.Command, args []string) {
		var summaries []jobs.JobSummary
		var err error

		if jobsLocal {
			summaries, err = listLocalJobs(args)
		} else {
			userFilter := jobsUser
			if userFilter == "me" {
				key, keyErr := auth.GetPubkey()
				if keyErr != nil {
					log.Fatal(keyErr)
				}
				userFilter = key
			}
			if len(args) > 0 && jobsCohort != "" {
				log.Fatalf("Cannot use both a sproutID argument and --cohort (-C)")
			}
			if jobsCohort != "" {
				// Resolve cohort to sprout list, then fetch jobs for each.
				members, cohortErr := client.ResolveCohort(jobsCohort)
				if cohortErr != nil {
					log.Fatalf("Failed to resolve cohort %q: %v", jobsCohort, cohortErr)
				}
				for _, sproutID := range members {
					sproutJobs, listErr := client.ListJobsForSprout(sproutID)
					if listErr != nil {
						log.Errorf("Failed to list jobs for %s: %v", sproutID, listErr)
						continue
					}
					summaries = append(summaries, sproutJobs...)
				}
			} else if len(args) > 0 {
				summaries, err = client.ListJobsForSprout(args[0])
			} else {
				summaries, err = client.ListJobs(jobsLimit, userFilter)
			}
		}
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, jsonErr := json.Marshal(summaries)
			if jsonErr != nil {
				log.Fatal(jsonErr)
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

func listLocalJobs(args []string) ([]jobs.JobSummary, error) {
	storePath, err := jobs.DefaultCLIStorePath()
	if err != nil {
		return nil, fmt.Errorf("determining CLI store path: %w", err)
	}
	store, err := jobs.NewCLIStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("opening CLI job store: %w", err)
	}

	userKey := jobsUser
	if userKey == "me" {
		key, keyErr := auth.GetPubkey()
		if keyErr != nil {
			return nil, fmt.Errorf("getting current user key: %w", keyErr)
		}
		userKey = key
	}

	var sproutFilter string
	if len(args) > 0 {
		sproutFilter = args[0]
	}

	return store.ListJobs(jobsLimit, userKey, sproutFilter)
}

var cmdJobsShow = &cobra.Command{
	Use:   "show <JID>",
	Short: "Show details of a specific job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jid := args[0]

		if jobsLocal {
			showLocalJob(jid)
			return
		}

		summary, err := client.GetJob(jid)
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, jsonErr := json.Marshal(summary)
			if jsonErr != nil {
				log.Fatal(jsonErr)
			}
			fmt.Println(string(b))
		default:
			printJobDetail(summary)
		}
	},
}

func showLocalJob(jid string) {
	storePath, err := jobs.DefaultCLIStorePath()
	if err != nil {
		log.Fatal(err)
	}
	store, err := jobs.NewCLIStore(storePath)
	if err != nil {
		log.Fatal(err)
	}

	summary, meta, err := store.GetJob(jid)
	if err != nil {
		log.Fatal(err)
	}

	switch outputMode {
	case "json":
		wrapper := map[string]interface{}{
			"summary": summary,
		}
		if meta != nil {
			wrapper["meta"] = meta
		}
		b, jsonErr := json.Marshal(wrapper)
		if jsonErr != nil {
			log.Fatal(jsonErr)
		}
		fmt.Println(string(b))
	default:
		if meta != nil {
			fmt.Printf("User:    %s\n", meta.UserKey)
			if meta.Recipe != "" {
				fmt.Printf("Recipe:  %s\n", meta.Recipe)
			}
		}
		printJobDetail(summary)
	}
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
	// Check whether any job has invoker info to decide column visibility.
	hasInvoker := false
	for _, s := range summaries {
		if s.InvokedBy != "" {
			hasInvoker = true
			break
		}
	}

	if hasInvoker {
		fmt.Printf("%-20s  %-24s  %-10s  %-12s  %5s  %5s  %5s  %s\n",
			"JID", "SPROUT", "STATUS", "USER", "OK", "FAIL", "SKIP", "STARTED")
		fmt.Println(strings.Repeat("-", 115))
	} else {
		fmt.Printf("%-20s  %-24s  %-10s  %5s  %5s  %5s  %s\n",
			"JID", "SPROUT", "STATUS", "OK", "FAIL", "SKIP", "STARTED")
		fmt.Println(strings.Repeat("-", 100))
	}

	for _, s := range summaries {
		statusStr := formatStatus(s.Status)
		started := "—"
		if !s.StartedAt.IsZero() {
			started = s.StartedAt.Format(time.RFC3339)
		}
		if hasInvoker {
			fmt.Printf("%-20s  %-24s  %-10s  %-12s  %5d  %5d  %5d  %s\n",
				truncate(s.JID, 20),
				truncate(s.SproutID, 24),
				statusStr,
				truncate(s.InvokedBy, 12),
				s.Succeeded,
				s.Failed,
				s.Skipped,
				started,
			)
		} else {
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
}

func printJobDetail(s *jobs.JobSummary) {
	fmt.Printf("Job:     %s\n", s.JID)
	fmt.Printf("Sprout:  %s\n", s.SproutID)
	fmt.Printf("Status:  %s\n", formatStatus(s.Status))
	if s.InvokedBy != "" {
		fmt.Printf("User:    %s\n", s.InvokedBy)
	}
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

var cmdJobsPurge = &cobra.Command{
	Use:   "purge",
	Short: "Remove old jobs from local CLI-side storage",
	Long: `Remove job files older than the specified number of hours from local storage.
Defaults to 720 hours (30 days).`,
	Run: func(cmd *cobra.Command, args []string) {
		storePath, err := jobs.DefaultCLIStorePath()
		if err != nil {
			log.Fatal(err)
		}
		store, err := jobs.NewCLIStore(storePath)
		if err != nil {
			log.Fatal(err)
		}

		dur := time.Duration(purgeOlderH) * time.Hour
		removed, err := store.Purge(dur)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Purged %d job(s) older than %dh.\n", removed, purgeOlderH)
	},
}

var cmdJobsStats = &cobra.Command{
	Use:   "stats",
	Short: "Show local CLI-side job store statistics",
	Run: func(cmd *cobra.Command, args []string) {
		storePath, err := jobs.DefaultCLIStorePath()
		if err != nil {
			log.Fatal(err)
		}
		store, err := jobs.NewCLIStore(storePath)
		if err != nil {
			log.Fatal(err)
		}

		stats, err := store.Stats()
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			b, jsonErr := json.Marshal(stats)
			if jsonErr != nil {
				log.Fatal(jsonErr)
			}
			fmt.Println(string(b))
		default:
			fmt.Printf("Jobs:    %d\n", stats.TotalJobs)
			fmt.Printf("Sprouts: %d\n", stats.TotalSprouts)
			fmt.Printf("Disk:    %s\n", humanizeBytes(stats.DiskBytes))
		}
	},
}

var (
	deleteLocal bool
)

var cmdJobsDelete = &cobra.Command{
	Use:   "delete <JID>",
	Short: "Delete a specific job",
	Long: `Delete a job by JID. By default, deletes from the farmer's server-side store.
Use --local to delete from local CLI-side storage instead.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jid := args[0]

		if deleteLocal {
			storePath, err := jobs.DefaultCLIStorePath()
			if err != nil {
				log.Fatal(err)
			}
			store, err := jobs.NewCLIStore(storePath)
			if err != nil {
				log.Fatal(err)
			}
			if err := store.DeleteJob(jid); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Deleted job %s from local storage.\n", jid)
			return
		}

		if err := client.DeleteJob(jid); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Deleted job %s from farmer.\n", jid)
	},
}

func humanizeBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func init() {
	cmdJobsList.Flags().IntVar(&jobsLimit, "limit", 50, "Maximum number of jobs to return")
	cmdJobsList.Flags().BoolVar(&jobsLocal, "local", false, "List jobs from local CLI-side storage instead of the farmer")
	cmdJobsList.Flags().StringVar(&jobsUser, "user", "", "Filter jobs by invoking user's pubkey (use 'me' for current user)")
	cmdJobsList.Flags().StringVarP(&jobsCohort, "cohort", "C", "", "Filter jobs to sprouts in a cohort")
	cmdJobsShow.Flags().BoolVar(&jobsLocal, "local", false, "Show job from local CLI-side storage instead of the farmer")
	cmdJobsWatch.Flags().IntVar(&watchTimeout, "timeout", 120, "Watch timeout in seconds")
	cmdJobsPurge.Flags().IntVar(&purgeOlderH, "older-than", 720, "Remove jobs older than this many hours (default 720 = 30 days)")
	cmdJobsDelete.Flags().BoolVar(&deleteLocal, "local", false, "Delete from local CLI-side storage instead of the farmer")
	cmdJobs.AddCommand(cmdJobsList)
	cmdJobs.AddCommand(cmdJobsShow)
	cmdJobs.AddCommand(cmdJobsWatch)
	cmdJobs.AddCommand(cmdJobsCancel)
	cmdJobs.AddCommand(cmdJobsPurge)
	cmdJobs.AddCommand(cmdJobsStats)
	cmdJobs.AddCommand(cmdJobsDelete)
	rootCmd.AddCommand(cmdJobs)
}
