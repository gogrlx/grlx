package main

import (
	"encoding/json"
	"time"

	log "github.com/taigrr/log-socket/log"

	"github.com/gogrlx/grlx/pki"
	. "github.com/gogrlx/grlx/types"

	nats "github.com/nats-io/nats.go"
)

func init() {
	log.SetLogLevel(log.LTrace)
	createConfigRoot()
	pki.SetupPKISprout()
}

func natsInit(nc *nats.EncodedConn) error {
	log.Debugf("Announcing on Farmer...")
	startup := Startup{}
	startup.Version.Authors = Authors
	startup.Version.BuildNo = BuildNo
	startup.Version.BuildTime = BuildTime
	startup.Version.GitCommit = GitCommit
	startup.Version.Package_ = Package
	startup.Version.Tag = Tag
	startup_event := "grlx.sprouts.announce." + sproutID
	b, _ := json.Marshal(startup)
	nc.Publish(startup_event, b)
	if err := nc.LastError(); err != nil {
		log.Fatal(err)
	} else {
		log.Tracef("Successfully published startup message on `%s`.", startup_event)
	}
	return nil
}

// Simple hardcoded setup to get onto NATS.
// Configures timeouts, and sets up logging for disconnection events
func setupConnOptions(opts []nats.Option) []nats.Option {
	totalWait := 10 * time.Minute
	reconnectDelay := time.Second
	opts = append(opts, nats.ReconnectWait(reconnectDelay))
	opts = append(opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
	opts = append(opts, nats.DisconnectHandler(func(nc *nats.Conn) {
		log.Errorf("Disconnected: will attempt reconnects for %.0fm", totalWait.Minutes())
	}))
	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		log.Debugf("Reconnected [%s]", nc.ConnectedUrl())
	}))
	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		log.Fatal("Exiting, no servers available")
	}))
	return opts
}
