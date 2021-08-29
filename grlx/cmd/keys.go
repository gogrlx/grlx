package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	//. "github.com/gogrlx/grlx/config"
	gpki "github.com/gogrlx/grlx/grlx/pki"
	"github.com/gogrlx/grlx/grlx/util"
	. "github.com/gogrlx/grlx/types"
	"github.com/spf13/cobra"
)

var targetAll bool
var noConfirm bool

// keysCmd represents the keys command
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Key management for Sprouts",
	Long:  `Subcommands allow for Sprout key CRUD.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	keysCmd.InheritedFlags().Set("target", "_")
	keysCmd.PersistentFlags().BoolVar(&noConfirm, "no-confirm", false, "Do not prompt for confirmation")
	keys_accept.Flags().BoolVarP(&targetAll, "all", "A", false, "Accept all unaccepted keys")
	keysCmd.AddCommand(keys_accept)
	keysCmd.AddCommand(keys_deny)
	keysCmd.AddCommand(keys_reject)
	keysCmd.AddCommand(keys_unaccept)
	keysCmd.AddCommand(keys_delete)
	keysCmd.AddCommand(keys_list)
	rootCmd.AddCommand(keysCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// keysCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// keysCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

var keys_accept = &cobra.Command{
	Use:   "accept 'key_id'",
	Short: "Accept a Sprout key by id.",
	Long:  `Allows a user to accept one or many keys by id.`,
	Run: func(cmd *cobra.Command, args []string) {
		if targetAll {
			keyList, err := gpki.ListKeys()
			//TODO: utility function for switched output mode errors
			if err != nil {
				switch outputMode {
				case "":
					fallthrough
				case "text":
					color.Red("Error: %v", err)
				case "json":
				case "yaml":
				}
				return
			}
			if !noConfirm {
				fmt.Printf("Accept all unaccepted keys? ")
				confirm, err := util.UserConfirmWithDefault(true)
				for err != nil {
					confirm, err = util.UserConfirmWithDefault(true)
				}
				if !confirm {
					return
				}
			}
			accepted := KeySet{Sprouts: []KeyManager{}}
			for _, id := range keyList.Unaccepted.Sprouts {
				ok, err := gpki.AcceptKey(id.SproutID)
				if ok {
					accepted.Sprouts = append(accepted.Sprouts, id)
				} else {
					switch outputMode {
					case "":
						fallthrough
					case "text":
						color.Red("Error: %v", err)
					case "json":
					case "yaml":
					}
					return
				}
			}
			switch outputMode {
			case "":
				fallthrough
			case "text":
				for _, id := range accepted.Sprouts {
					fmt.Printf("Key %s accepted.\n", id.SproutID)
				}
			case "json":
			case "yaml":
			}
			return
		}
		if len(args) < 1 {
			cmd.Help()
			return
		}
		keyID := args[0]
		if !noConfirm {
			fmt.Printf("Accept %s ", keyID)
			confirm, err := util.UserConfirmWithDefault(true)
			for err != nil {
				confirm, err = util.UserConfirmWithDefault(true)
			}
			if !confirm {
				return
			}
		}
		ok, err := gpki.AcceptKey(keyID)
		//TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case ErrSproutIDNotFound:
				log.Fatalf("Sprout %s does not exist.", keyID)
			case ErrAlreadyAccepted:
				log.Fatalf("Sprout %s has already been accepted.", keyID)
			default:
				panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(ok)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if ok {
				fmt.Printf("%s Accepted.\n", keyID)
				return
			}
			color.Red("%s could not be Accepted!\n", keyID)
			os.Exit(1)
		case "yaml":
			//TODO implement YAML
		}

	},
}
var keys_list = &cobra.Command{
	Use:   "list",
	Short: "List the Sprout keys available on the farmer.",
	Run: func(cmd *cobra.Command, args []string) {
		keys, err := gpki.ListKeys()
		//TODO: output error message in correct outputMode
		// for example, invalid cert for interface
		// or 'unsigned'
		if err != nil {
			panic(err)
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(keys)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			color.Green("Accepted:")
			for _, key := range keys.Accepted.Sprouts {
				color.Green(key.SproutID)
			}
			color.Blue("Rejected:")
			for _, key := range keys.Rejected.Sprouts {
				color.Blue(key.SproutID)
			}
			color.Red("Denied:")
			for _, key := range keys.Denied.Sprouts {
				color.Red(key.SproutID)
			}
			color.Yellow("Unaccepted:")
			for _, key := range keys.Unaccepted.Sprouts {
				color.Yellow(key.SproutID)
			}
			return
		case "yaml":
			//TODO implement YAML
		}

	},
}
var keys_delete = &cobra.Command{
	Use:   "delete [sprout id]",
	Short: "Delete a Sprout key from the Farmer by id.",
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		if !noConfirm {
			fmt.Printf("Delete %s ", keyID)
			confirm, err := util.UserConfirmWithDefault(true)
			for err != nil {
				confirm, err = util.UserConfirmWithDefault(true)
			}
			if !confirm {
				return
			}
		}
		ok, err := gpki.DeleteKey(keyID)
		//TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case ErrSproutIDNotFound:
				log.Fatalf("Sprout %s does not exist.", keyID)
			default:
				panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(ok)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if ok {
				fmt.Printf("%s Deleted.\n", keyID)
				return
			}
			color.Red("%s could not be Deleted!\n", keyID)
			os.Exit(1)
		case "yaml":
			//TODO implement YAML
		}
	},
}
var keys_unaccept = &cobra.Command{
	Use:   "unaccept [sprout id]",
	Short: "Move a Sprout key to the unaccepted group by id.",
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		if !noConfirm {
			fmt.Printf("Unaccept %s ", keyID)
			confirm, err := util.UserConfirmWithDefault(true)
			for err != nil {
				confirm, err = util.UserConfirmWithDefault(true)
			}
			if !confirm {
				return
			}
		}
		ok, err := gpki.UnacceptKey(keyID)
		//TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case ErrSproutIDNotFound:
				log.Fatalf("Sprout %s does not exist.", keyID)
			case ErrAlreadyUnaccepted:
				log.Fatalf("Sprout %s has already been unaccepted.", keyID)

			default:
				panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(ok)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if ok {
				fmt.Printf("%s Unaccepted.\n", keyID)
				return
			}
			color.Red("%s could not be Unaccepted!\n", keyID)
			os.Exit(1)
		case "yaml":
			//TODO implement YAML
		}
	},
}
var keys_reject = &cobra.Command{
	Use:   "reject",
	Short: "Move a Sprout key to the rejected group by id.",
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		if !noConfirm {
			fmt.Printf("Reject %s ", keyID)
			confirm, err := util.UserConfirmWithDefault(true)
			for err != nil {
				confirm, err = util.UserConfirmWithDefault(true)
			}
			if !confirm {
				return
			}
		}
		ok, err := gpki.RejectKey(keyID)
		//TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case ErrSproutIDNotFound:
				log.Fatalf("Sprout %s does not exist.", keyID)
			case ErrAlreadyRejected:
				log.Fatalf("Sprout %s has already been rejected.", keyID)

			default:
				panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(ok)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if ok {
				fmt.Printf("%s Rejected.\n", keyID)
				return
			}
			color.Red("%s could not be Rejected!\n", keyID)
			os.Exit(1)
		case "yaml":
			//TODO implement YAML
		}
	},
}
var keys_deny = &cobra.Command{
	Use:   "deny",
	Short: "Move a Sprout key to the denied group by id.",
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		if !noConfirm {
			fmt.Printf("Deny %s ", keyID)
			confirm, err := util.UserConfirmWithDefault(true)
			for err != nil {
				confirm, err = util.UserConfirmWithDefault(true)
			}
			if !confirm {
				return
			}
		}
		ok, err := gpki.DenyKey(keyID)
		//TODO: output error message in correct outputMode
		if err != nil {
			switch err {
			case ErrSproutIDNotFound:
				log.Fatalf("Sprout %s does not exist.", keyID)
			case ErrAlreadyDenied:
				log.Fatalf("Sprout %s has already been denied.", keyID)

			default:
				panic(err)
			}
		}
		switch outputMode {
		case "json":
			jw, _ := json.Marshal(ok)
			fmt.Println(string(jw))
			return
		case "":
			fallthrough
		case "text":
			if ok {
				fmt.Printf("%s Denied.\n", keyID)
				return
			}
			color.Red("%s could not be Denied!\n", keyID)
			os.Exit(1)
		case "yaml":
			//TODO implement YAML
		}
	},
}
