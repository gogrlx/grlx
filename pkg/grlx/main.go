package main

import (
	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/pkg/grlx/cmd"
	"github.com/gogrlx/grlx/types"
)

func init() {
	log.SetLogLevel(log.LDebug)
}

const DocumentationURL = "https://grlx.org"

var (
	Arch      string
	BuildTime string
	GitCommit string
	Tag       string
)

func main() {
	defer log.Flush()
	cmd.Execute(types.Version{
		Arch:      Arch,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		Tag:       Tag,
	})
}
