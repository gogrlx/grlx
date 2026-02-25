package main

import (
	"bytes"
	"encoding/json"
	"runtime"

	log "github.com/taigrr/log-socket/log"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/pki"

	nats "github.com/nats-io/nats.go"
)

func init() {
	createConfigRoot()
	pki.SetupPKISprout()
}

func natsInit(nc *nats.Conn) error {
	log.Debugf("Announcing on Farmer...")
	startup := config.Startup{}
	startup.Version.Arch = runtime.GOARCH
	startup.Version.Compiler = runtime.Version()
	startup.Version.GitCommit = GitCommit
	startup.Version.Tag = Tag
	startup.SproutID = sproutID
	startupEvent := "grlx.sprouts.announce." + sproutID
	b, _ := json.Marshal(startup)
	err := nc.Publish(startupEvent, b)
	if err != nil {
		return err
	}
	if err = nc.LastError(); err != nil {
		log.Fatal(err)
	} else {
		log.Tracef("Successfully published startup message on `%s`.", startupEvent)
	}

	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".cmd.run", func(m *nats.Msg) {
		var cmdRun apitypes.CmdRun
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&cmdRun)
		log.Trace(cmdRun)
		results, err := cmd.SRun(cmdRun)
		if err != nil {
			log.Error(err)
		}
		resultsB, err := json.Marshal(results)
		if err != nil {
			log.Error(err)
		}
		m.Respond(resultsB)
	})
	if err != nil {
		return err
	}
	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(m *nats.Msg) {
		var ping apitypes.PingPong
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
		var rEnvelope cook.RecipeEnvelope
		json.NewDecoder(bytes.NewBuffer(m.Data)).Decode(&rEnvelope)
		log.Trace(rEnvelope)
		ackB, _ := json.Marshal(cook.Ack{Acknowledged: true, JobID: rEnvelope.JobID})
		m.Respond(ackB)
		go func() {
			err = cook.CookRecipeEnvelope(rEnvelope)
			if err != nil {
				log.Error(err)
			}
		}()
	})
	if err != nil {
		return err
	}
	return nil
}
