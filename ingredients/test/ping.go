package test

import (
	"time"

	"github.com/gogrlx/grlx/types"
)

// TODO allow selector to be more than an ID
func FPing(target types.KeyManager, ping types.PingPong) (types.PingPong, error) {
	topic := "grlx.sprouts." + target.SproutID + ".test.ping"
	ping.Ping = true
	ping.Pong = false
	var pong types.PingPong
	err := ec.Request(topic, ping, &pong, time.Second*15)
	return pong, err
}

func SPing(ping types.PingPong) (types.PingPong, error) {
	ping.Pong = true
	return ping, nil
}
