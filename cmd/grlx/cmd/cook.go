package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/api/client"
	"github.com/gogrlx/grlx/types"
)

var async bool

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
		ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
		if err != nil {
			log.Fatal(err)
		}
		finished := make(chan struct{})
		completions := make(chan types.SproutStepCompletion)
		topic := fmt.Sprintf("grlx.cook.*.%s", jid)
		completionSteps := make(map[string][]types.StepCompletion)
		sub, err := ec.Subscribe(topic, func(msg *nats.Msg) {
			var step types.StepCompletion
			err := json.Unmarshal(msg.Data, &step)
			if err != nil {
				log.Printf("Error unmarshalling message: %v\n", err)
				return
			}
			subComponents := strings.Split(msg.Subject, ".")
			sproutID := subComponents[2]

			completions <- types.SproutStepCompletion{SproutID: sproutID, CompletedStep: step}
			printTex.Lock()
			defer printTex.Unlock()
			switch outputMode {
			case "json":
				jw, _ := json.Marshal(results)
				fmt.Println(string(jw))
				return
			case "":
				fallthrough
			case "text":
				//			for keyID, result := range results.Results {
				//				jw, err := json.Marshal(result)
				//				if err != nil {
				//					color.Red("%s: \n returned an invalid message!\n", keyID)
				//					continue
				//				}
				//				var value types.CmdRun
				//				err = json.NewDecoder(bytes.NewBuffer(jw)).Decode(&value)
				//				if err != nil {
				//					color.Red("%s returned an invalid message!\n", keyID)
				//					continue
				//				}
				//				if value.ErrCode != 0 {
				//					color.Red("%s:\n", keyID)
				//				} else {
				//					fmt.Printf("%s:\n", keyID)
				//				}
				//				if noerr {
				//					fmt.Printf("%s\n", value.Stdout)
				//				} else {
				//					fmt.Printf("%s%s\n", value.Stdout, value.Stderr)
				//				}
				//			}
			}
		})
		if err != nil {
			log.Printf("Error subscribing to %s: %v\n", topic, err)
			log.Fatal(err)
		}
		ec.Publish(fmt.Sprintf("grlx.farmer.cook.trigger.%s", jid), types.TriggerMsg{JID: jid})
		timeout := time.After(30 * time.Second)
	waitLoop:
		for {
			select {
			case completion := <-completions:
				completionSteps[completion.SproutID] = append(completionSteps[completion.SproutID], completion.CompletedStep)
				timeout = time.After(30 * time.Second)
			case <-finished:
				break waitLoop
			case <-timeout:
				log.Error("Cooking timed out after 30 seconds.")
				break waitLoop
			}
		}
		defer sub.Unsubscribe()
		defer nc.Flush()
		<-finished
	},
}

func init() {
	cmdCook.Flags().StringVarP(&environment, "environment", "E", "", "")
	cmdCook.Flags().BoolVar(&async, "async", false, "Don't print any output, just return the JID to look up results later")
	cmdCook.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "list of sprouts to target")
	cmdCook.MarkPersistentFlagRequired("target")
	rootCmd.AddCommand(cmdCook)
}
