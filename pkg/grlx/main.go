package main

import (
	"runtime"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/pkg/grlx/cmd"
	"github.com/gogrlx/grlx/types"
)

func init() {
	log.SetLogLevel(log.LDebug)
}

const DocumentationURL = "https://grlx.org"

var (
	BuildTime string
	GitCommit string
	Tag       string
)

func main() {
	defer log.Flush()
	cmd.Execute(types.Version{
		Arch:      runtime.GOOS,
		BuildTime: BuildTime,
		Compiler:  runtime.Version(),
		GitCommit: GitCommit,
		Tag:       Tag,
	})
}
