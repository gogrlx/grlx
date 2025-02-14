package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/api/client"
	"github.com/gogrlx/grlx/types"
)

var (
	async       bool
	cookTimeout int
)

// cmdCmd represents the cmd command
var cookCmd = &cobra.Command{
	Use:   "cook",
	Short: "Cook a recipe on a Sprout or a list of Sprouts",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Help()
	},
}

var cmdCook = &cobra.Command{
	Use:   "cook <recipe> -T <target> [and optional args]...",
	Short: "Cook a recipe against a target and see the output locally.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Help()
			return
		}
		var cmdCook types.CmdCook
		cmdCook.Recipe = types.RecipeName(args[0])
		cmdCook.Async = async
		cmdCook.Env = environment
		//	cmdCook.Test = test

		results, err := client.Cook(sproutTarget, cmdCook)
		if err != nil {
			switch err {
			case types.ErrSproutIDNotFound:
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
		completions := make(chan types.SproutStepCompletion)
		topic := fmt.Sprintf("grlx.cook.*.%s", jid)
		completionSteps := make(map[string][]types.StepCompletion)
		sub, err := nc.Subscribe(topic, func(msg *nats.Msg) {
			var step types.StepCompletion
			err := json.Unmarshal(msg.Data, &step)
			if err != nil {
				log.Errorf("Error unmarshalling message: %v\n", err)
				return
			}
			subComponents := strings.Split(msg.Subject, ".")
			sproutID := subComponents[2]

			completions <- types.SproutStepCompletion{SproutID: sproutID, CompletedStep: step}
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
				case types.StepCompleted:
					b.WriteString(color.GreenString(fmt.Sprintf("\tResult: %s\n", "Success")))
				case types.StepFailed:
					b.WriteString(color.RedString(fmt.Sprintf("\tResult: %s\n", "Failure")))
				default:
					// TODO add a status for skipped steps
					b.WriteString(color.YellowString(fmt.Sprintf("\tResult: %s\n", "Unknown")))
				}
				b.WriteString("\tExecution Notes: \n")
				for _, change := range step.Changes {
					b.WriteString(fmt.Sprintf("\t\t%s\n", change))
				}
				// TODO add started and duration
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
		// TODO convert this to a request and get back the list of targeted sprouts
		triggerMsg := types.TriggerMsg{JID: jid}
		b, _ := json.Marshal(triggerMsg)
		nc.Publish(fmt.Sprintf("grlx.farmer.cook.trigger.%s", jid), b)
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
				errors := []string{}
				for _, step := range v {
					if step.CompletionStatus == types.StepCompleted {
						successes++
					} else if step.CompletionStatus == types.StepFailed {
						failures++
					}
					if step.Error != nil {
						errors = append(errors, step.Error.Error())
					}
				}
				fmt.Printf("Summary for %s, JID %s:\n", k, jid)
				fmt.Printf("\tSuccesses:\t%d\n", successes)
				fmt.Printf("\tFailures:\t%d\n", failures)
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
	cmdCook.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "list of sprouts to target")
	cmdCook.Flags().IntVar(&cookTimeout, "cook-timeout", 30, "Cancel cook execution and return after X seconds")
	cmdCook.MarkPersistentFlagRequired("target")
	rootCmd.AddCommand(cmdCook)
}
