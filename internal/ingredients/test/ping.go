package test

import (
	"encoding/json"
	"time"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// TODO allow selector to be more than an ID
func FPing(target pki.KeyManager, ping apitypes.PingPong) (apitypes.PingPong, error) {
	topic := "grlx.sprouts." + target.SproutID + ".test.ping"
	ping.Ping = true
	ping.Pong = false
	var pong apitypes.PingPong
	b, _ := json.Marshal(ping)
	msg, err := nc.Request(topic, b, time.Second*15)
	if err != nil {
		err = json.Unmarshal(msg.Data, &pong)
	}
	return pong, err
}

func SPing(ping apitypes.PingPong) (apitypes.PingPong, error) {
	ping.Pong = true
	return ping, nil
}
