package main

import (
	"encoding/json"
	"runtime"

	log "github.com/gogrlx/grlx/v2/internal/log"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/facts"
	"github.com/gogrlx/grlx/v2/internal/ingredients/cmd"
	"github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/shell"
	"github.com/gogrlx/grlx/v2/internal/sproutnats"

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

	// Publish system facts on startup.
	sysFacts := facts.Collect()
	sysFacts.SproutID = sproutID
	factsB, _ := json.Marshal(sysFacts)
	if pubErr := nc.Publish("grlx.sprouts."+sproutID+".facts", factsB); pubErr != nil {
		log.Errorf("failed to publish system facts: %v", pubErr)
	}

	// Respond to on-demand facts requests from the farmer.
	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".facts.request", func(m *nats.Msg) {
		fresh := facts.Collect()
		fresh.SproutID = sproutID
		b, _ := json.Marshal(fresh)
		m.Respond(b)
	})
	if err != nil {
		return err
	}

	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".cmd.run", func(m *nats.Msg) {
		cmdRun, errorResponse, decodeErr := sproutnats.DecodeCmdRun(m.Data)
		if decodeErr != nil {
			log.Errorf("invalid cmd.run request: %v", decodeErr)
			resultsB, _ := json.Marshal(errorResponse)
			m.Respond(resultsB)
			return
		}
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
		ping, errorResponse, decodeErr := sproutnats.DecodePing(m.Data)
		if decodeErr != nil {
			log.Errorf("invalid test.ping request: %v", decodeErr)
			pongB, _ := json.Marshal(errorResponse)
			m.Respond(pongB)
			return
		}
		log.Trace(ping)
		pong, _ := test.SPing(ping)
		pongB, _ := json.Marshal(pong)
		m.Respond(pongB)
	})
	if err != nil {
		return err
	}
	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".cook", func(m *nats.Msg) {
		rEnvelope, errorResponse, decodeErr := sproutnats.DecodeCook(m.Data)
		if decodeErr != nil {
			log.Errorf("invalid cook request: %v", decodeErr)
			ackB, _ := json.Marshal(errorResponse)
			m.Respond(ackB)
			return
		}
		log.Trace(rEnvelope)
		ackB, _ := json.Marshal(cook.Ack{Acknowledged: true, JobID: rEnvelope.JobID})
		m.Respond(ackB)
		go func() {
			if cookErr := cook.CookRecipeEnvelope(rEnvelope); cookErr != nil {
				log.Error(cookErr)
			}
		}()
	})
	if err != nil {
		return err
	}

	// Interactive shell sessions.
	_, err = nc.Subscribe("grlx.sprouts."+sproutID+".shell.start", func(m *nats.Msg) {
		shell.HandleShellStart(nc, m)
	})
	if err != nil {
		return err
	}

	return nil
}
