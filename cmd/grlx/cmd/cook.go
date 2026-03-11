package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

var (
	async       bool
	cookTimeout int
	testMode    bool
)

var cmdCook = &cobra.Command{
	Use:   "cook <recipe> (-T <target> | -C <cohort>) [flags]",
	Short: "Cook a recipe against a target or cohort and see the output locally.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Help()
			return
		}
		var cmdCook apitypes.CmdCook
		cmdCook.Recipe = cook.RecipeName(args[0])
		cmdCook.Async = async
		cmdCook.Env = environment
		cmdCook.Test = testMode

		effectiveTarget, err := resolveEffectiveTarget()
		if err != nil {
			log.Fatal(err)
		}
		results, err := client.Cook(effectiveTarget, cmdCook)
		if err != nil {
			switch err {
			case pki.ErrSproutIDNotFound:
				log.Fatalf("A targeted Sprout does not exist or is not accepted.")
			default:
				log.Fatal(err)
			}
		}
		// topic: grlx.cook."+envelope.JobID+"."+pki.GetSproutID()
		jid := results.JID
		nc, err := client.NewNatsClient()
		if err != nil {
			log.Fatal(err)
		}
		finished := make(chan struct{}, 1)
		completions := make(chan cook.SproutStepCompletion)
		topic := fmt.Sprintf("grlx.cook.*.%s", jid)
		completionSteps := make(map[string][]cook.StepCompletion)
		sub, err := nc.Subscribe(topic, func(msg *nats.Msg) {
			var step cook.StepCompletion
			err := json.Unmarshal(msg.Data, &step)
			if err != nil {
				log.Errorf("Error unmarshalling message: %v\n", err)
				return
			}
			subComponents := strings.Split(msg.Subject, ".")
			sproutID := subComponents[2]

			completions <- cook.SproutStepCompletion{SproutID: sproutID, CompletedStep: step}
			if string(step.ID) == fmt.Sprintf("completed-%s", jid) {
				return
			} else if string(step.ID) == fmt.Sprintf("start-%s", jid) {
				return
			}

			switch outputMode {
			case "json":
			case "":
				fallthrough
			case "text":
				var b strings.Builder
				b.WriteString(fmt.Sprintf("%s::%s\n", sproutID, jid))
				b.WriteString(fmt.Sprintf("ID: %s\n", step.ID))
				switch step.CompletionStatus {
				case cook.StepCompleted:
					b.WriteString(color.GreenString(fmt.Sprintf("\tResult: %s\n", "Success")))
				case cook.StepFailed:
					b.WriteString(color.RedString(fmt.Sprintf("\tResult: %s\n", "Failure")))
				case cook.StepSkipped:
					b.WriteString(color.YellowString(fmt.Sprintf("\tResult: %s\n", "Skipped")))
				default:
					b.WriteString(color.YellowString(fmt.Sprintf("\tResult: %s\n", "Unknown")))
				}
				b.WriteString("\tExecution Notes: \n")
				for _, change := range step.Changes {
					b.WriteString(fmt.Sprintf("\t\t%s\n", change))
				}
				if !step.Started.IsZero() {
					b.WriteString(fmt.Sprintf("\tStarted:  %s\n", step.Started.Format(time.RFC3339)))
					b.WriteString(fmt.Sprintf("\tDuration: %s\n", step.Duration.Round(time.Millisecond)))
				}
				b.WriteString("----------\n")
				printTex.Lock()
				fmt.Print(b.String())
				printTex.Unlock()
			}
		})
		if err != nil {
			log.Printf("Error subscribing to %s: %v\n", topic, err)
			log.Fatal(err)
		}
		triggerMsg := config.TriggerMsg{JID: jid}
		b, _ := json.Marshal(triggerMsg)
		triggerReply, err := nc.Request(fmt.Sprintf("grlx.farmer.cook.trigger.%s", jid), b, 15*time.Second)
		if err != nil {
			log.Fatalf("Failed to trigger cook: %v", err)
		}
		var targetedSprouts []string
		if err := json.Unmarshal(triggerReply.Data, &targetedSprouts); err == nil && len(targetedSprouts) > 0 {
			fmt.Printf("Cooking on %d sprout(s): %s\n", len(targetedSprouts), strings.Join(targetedSprouts, ", "))
		}

		// Record job locally with per-user tracking.
		if cliStorePath, pathErr := jobs.DefaultCLIStorePath(); pathErr == nil {
			if cliStore, storeErr := jobs.NewCLIStore(cliStorePath); storeErr == nil {
				userKey, _ := auth.GetPubkey()
				cliListener := jobs.NewCLIListener(cliStore, nc, userKey)
				cliListener.RecordJobInit(jid, string(cmdCook.Recipe), targetedSprouts)
				if subErr := cliListener.SubscribeJob(jid); subErr != nil {
					log.Errorf("CLI job store: failed to subscribe: %v", subErr)
				} else {
					defer cliListener.Stop()
				}
			}
		}

		localTimeout := time.After(time.Duration(cookTimeout) * time.Second)
		dripTimeout := time.After(120 * time.Second)
		concurrent := 0
		defer sub.Unsubscribe()
		defer nc.Flush()
	waitLoop:
		for {
			select {
			case completion := <-completions:
				if string(completion.CompletedStep.ID) == fmt.Sprintf("start-%s", jid) {
					concurrent++
				}
				if string(completion.CompletedStep.ID) == fmt.Sprintf("completed-%s", jid) {
					// waitgroups are not necesary here because we are looping sequentially over a channel
					concurrent--
				}
				if concurrent == 0 {
					dripTimeout = time.After(time.Second / 10)
				}

				completionSteps[completion.SproutID] = append(completionSteps[completion.SproutID], completion.CompletedStep)
				localTimeout = time.After(time.Duration(cookTimeout) * time.Second)
			case <-finished:
				break waitLoop
			case <-dripTimeout:
				finished <- struct{}{}
				break waitLoop
			case <-localTimeout:
				color.Red(fmt.Sprintf("Cooking timed out after %d seconds.", cookTimeout))
				finished <- struct{}{}
				break waitLoop
			}
		}
		switch outputMode {
		case "json":
			wrapper := map[string]interface{}{}
			wrapper["jid"] = jid
			wrapper["sprouts"] = completionSteps
			jsonBytes, err := json.Marshal(wrapper)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(jsonBytes))
		case "":
			fallthrough
		case "text":
			for k, v := range completionSteps {
				successes := -2 // -2 because we don't count the start and completed steps
				failures := 0
				skipped := 0
				errors := []string{}
				for _, step := range v {
					switch step.CompletionStatus {
					case cook.StepCompleted:
						successes++
					case cook.StepFailed:
						failures++
					case cook.StepSkipped:
						skipped++
					}
					if step.Error != nil {
						errors = append(errors, step.Error.Error())
					}
				}
				fmt.Printf("Summary for %s, JID %s:\n", k, jid)
				fmt.Printf("\tSuccesses:\t%d\n", successes)
				fmt.Printf("\tFailures:\t%d\n", failures)
				if skipped > 0 {
					fmt.Printf("\tSkipped:\t%d\n", skipped)
				}
				fmt.Printf("\tErrors:\t\t%d\n", len(errors))
				for _, err := range errors {
					fmt.Printf("\t\t%s\n", err)
				}
			}
		}
	},
}

func init() {
	cmdCook.Flags().StringVarP(&environment, "environment", "E", "", "")
	cmdCook.Flags().BoolVar(&async, "async", false, "Don't print any output, just return the JID to look up results later")
	addTargetFlags(cmdCook)
	cmdCook.Flags().IntVar(&cookTimeout, "cook-timeout", 30, "Cancel cook execution and return after X seconds")
	cmdCook.Flags().BoolVar(&testMode, "test", false, "Run in test mode (dry run without applying changes)")
	rootCmd.AddCommand(cmdCook)
}
