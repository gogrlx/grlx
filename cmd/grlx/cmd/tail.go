package cmd

import (
	"fmt"
	"log"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/api/client"
)

var printTex sync.Mutex

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Tail the farmer's NATS bus",
	Run: func(cmd *cobra.Command, args []string) {
		nc, err := client.NewNatsClient()
		if err != nil {
			log.Fatal(err)
		}
		ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
		if err != nil {
			log.Fatal(err)
		}
		sub, err := ec.Subscribe("grlx.>", func(msg *nats.Msg) {
			printTex.Lock()
			fmt.Println(msg.Subject)
			fmt.Println(string(msg.Data))
			printTex.Unlock()
		})
		if err != nil {
			log.Fatal(err)
		}
		sub2, err := ec.Subscribe("_INBOX.>", func(msg *nats.Msg) {
			printTex.Lock()
			fmt.Println(msg.Subject)
			fmt.Println(string(msg.Data))
			printTex.Unlock()
		})
		if err != nil {
			log.Fatal(err)
		}
		defer sub.Unsubscribe()
		defer sub2.Unsubscribe()
		defer nc.Flush()
		select {}
	},
}

func init() {
	rootCmd.AddCommand(tailCmd)
}
