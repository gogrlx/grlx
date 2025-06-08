package main

import (
	"runtime"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/v2/cmd/grlx/cmd"
	"github.com/gogrlx/grlx/v2/types"
)

func init() {
	log.SetLogLevel(log.LError)
}

const DocumentationURL = "https://docs.grlx.dev"

var (
	GitCommit string
	Tag       string
)

func main() {
	defer log.Flush()
	cmd.Execute(types.Version{
		Arch:      runtime.GOOS,
		Compiler:  runtime.Version(),
		GitCommit: GitCommit,
		Tag:       Tag,
	})
}
