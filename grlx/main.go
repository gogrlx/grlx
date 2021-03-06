package main

import (
	"github.com/gogrlx/grlx/grlx/cmd"
	log "github.com/taigrr/log-socket/log"
)

func init() {
	log.SetLogLevel(log.LDebug)
}

func main() {
	defer log.Flush()
	cmd.Execute()
}
