package test

import (
	"encoding/json"
	"time"

	"github.com/gogrlx/grlx/v2/types"
)

// TODO allow selector to be more than an ID
func FPing(target types.KeyManager, ping types.PingPong) (types.PingPong, error) {
	topic := "grlx.sprouts." + target.SproutID + ".test.ping"
	ping.Ping = true
	ping.Pong = false
	var pong types.PingPong
	b, _ := json.Marshal(ping)
	msg, err := nc.Request(topic, b, time.Second*15)
	if err != nil {
		err = json.Unmarshal(msg.Data, &pong)
	}
	return pong, err
}

func SPing(ping types.PingPong) (types.PingPong, error) {
	ping.Pong = true
	return ping, nil
}
