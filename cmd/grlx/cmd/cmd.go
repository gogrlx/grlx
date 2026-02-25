package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fatih/color"
	gcmd "github.com/gogrlx/grlx/v2/cmd/grlx/ingredients/cmd"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/spf13/cobra"
)

var (
	environment string
	user        string
	noerr       bool
	cwd         string
	timeout     int
	path        string
)

// cmdCmd represents the cmd command
var cmdCmd = &cobra.Command{
	Use:   "cmd",
	Short: "Collection of utilities for running commands on Sprouts on the fly",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var cmdCmdRun = &cobra.Command{
	Use:   "run command [and optional args]...",
	Short: "Run a command remotely and see the output locally.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			cmd.Help()
			return
		}
		var command apitypes.CmdRun
		command.Command = args[0]
		if len(args) > 1 {
			command.Args = args[1:]
		}
		command.CWD = cwd
		command.Timeout = time.Second * time.Duration(timeout)
		command.Env = make(apitypes.EnvVar)
		for _, pair := range strings.Split(environment, " ") {
			if strings.ContainsRune(pair, '=') {
				kv := strings.SplitN(pair, "=", 2)
				command.Env[kv[0]] = kv[1]
			}
		}
		command.Path = path
		command.RunAs = user
		results, err := gcmd.FRun(sproutTarget, command)
		if err != nil {
			switch err {
			case pki.ErrSproutIDNotFound:
				log.Fatalf("A targeted Sprout does not exist or is not accepted..")
			default:
				// TODO: handle endpoint timeouts here
				// TODO: Error running command on the Sprout: nats: no responders available for request  run.go:65
				log.Panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(results)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			for keyID, result := range results.Results {
				jw, err := json.Marshal(result)
				if err != nil {
					color.Red("%s: \n returned an invalid message!\n", keyID)
					continue
				}
				var value apitypes.CmdRun
				err = json.NewDecoder(bytes.NewBuffer(jw)).Decode(&value)
				if err != nil {
					color.Red("%s returned an invalid message!\n", keyID)
					continue
				}
				if value.ErrCode != 0 {
					color.Red("%s:\n", keyID)
				} else {
					fmt.Printf("%s:\n", keyID)
				}
				if noerr {
					fmt.Printf("%s\n", value.Stdout)
				} else {
					fmt.Printf("%s%s\n", value.Stdout, value.Stderr)
				}
			}
		}
	},
}

func init() {
	cmdCmdRun.Flags().StringVarP(&environment, "environment", "E", "", "List of space-separated key=value OS Environment Variables")
	cmdCmdRun.Flags().BoolVar(&noerr, "noerr", false, "Don't print out the stderr from the command output")
	cmdCmdRun.Flags().StringVarP(&user, "runas", "u", "", "If running as a sudoer, run the command as another user")
	cmdCmdRun.Flags().StringVarP(&cwd, "cwd", "w", "", "Current working directory to run the command in")
	cmdCmdRun.Flags().IntVar(&timeout, "timeout", 30, "Cancel command execution and return after X seconds")
	cmdCmdRun.Flags().StringVarP(&path, "path", "p", "", "Prepend a folder to the PATH before execution")
	cmdCmd.PersistentFlags().StringVarP(&sproutTarget, "target", "T", "", "list of sprouts to target")
	cmdCmd.MarkPersistentFlagRequired("target")
	cmdCmd.AddCommand(cmdCmdRun)
	rootCmd.AddCommand(cmdCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// cmdCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cmdCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
