package main

import (
	"bytes"
	"encoding/json"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/ingredients/cmd"
	"github.com/gogrlx/grlx/ingredients/test"
	"github.com/gogrlx/grlx/pki"
	"github.com/gogrlx/grlx/types"

	nats "github.com/nats-io/nats.go"
)

func init() {
	log.SetLogLevel(log.LTrace)
	createConfigRoot()
	pki.SetupPKISprout()
}

func natsInit(nc *nats.EncodedConn) error {
	log.Debugf("Announcing on Farmer...")
	startup := types.Startup{}
	startup.Version.Authors = Authors
	startup.Version.BuildNo = BuildNo
	startup.Version.BuildTime = BuildTime
	startup.Version.GitCommit = GitCommit
	startup.Version.Package_ = Package
	startup.Version.Tag = Tag
	startupEvent := "grlx.sprouts.announce." + sproutID
	b, _ := json.Marshal(startup)
	nc.Publish(startupEvent, b)
	if err := nc.LastError(); err != nil {
		log.Fatal(err)
	} else {
		log.Tracef("Successfully published startup message on `%s`.", startupEvent)
	}

	nc.Subscribe("grlx.sprouts."+sproutID+".cmd.run", func(m *nats.Msg) {
		var cmdRun types.CmdRun
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&cmdRun)
		results, _ := cmd.SRun(cmdRun)
		resultsB, _ := json.Marshal(results)
		m.Respond(resultsB)
	})
	nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(m *nats.Msg) {
		var ping types.PingPong
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&ping)
		pong, _ := test.SPing(ping)
		pongB, _ := json.Marshal(pong)
		m.Respond(pongB)
	})
	return nil
}
