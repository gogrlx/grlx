package main

import (
	"bytes"
	"encoding/json"
	"runtime"

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
	startup.Version.Arch = runtime.GOARCH
	startup.Version.Compiler = runtime.Version()
	startup.Version.GitCommit = GitCommit
	startup.Version.Tag = Tag
	startup.SproutID = sproutID
	startupEvent := "grlx.sprouts.announce." + sproutID
	err := nc.Publish(startupEvent, startup)
	if err != nil {
		return err
	}
	if err = nc.LastError(); err != nil {
		log.Fatal(err)
	} else {
		log.Tracef("Successfully published startup message on `%s`.", startupEvent)
	}

	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".cmd.run", func(m *nats.Msg) {
		var cmdRun types.CmdRun
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&cmdRun)
		log.Trace(cmdRun)
		results, _ := cmd.SRun(cmdRun)
		resultsB, _ := json.Marshal(results)
		m.Respond(resultsB)
	})
	if err != nil {
		return err
	}
	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(m *nats.Msg) {
		var ping types.PingPong
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&ping)
		log.Trace(ping)
		pong, _ := test.SPing(ping)
		pongB, _ := json.Marshal(pong)
		m.Respond(pongB)
	})
	if err != nil {
		return err
	}
	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(m *nats.Msg) {
		var rEnvelope types.RecipeEnvelope
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&rEnvelope)
		log.Trace(rEnvelope)
		ackB, _ := json.Marshal(types.Ack{Acknowledged: true, JobID: rEnvelope.JobID})
		m.Respond(ackB)
	})
	if err != nil {
		return err
	}
	return nil
}
