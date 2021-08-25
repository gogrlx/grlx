package test

import (
	"time"

	. "github.com/gogrlx/grlx/types"
)

// TODO allow selector to be more than an ID
func FPing(target KeyManager, ping PingPong) (bool, error) {

	topic := "grlx.sprouts." + target.SproutID + ".test.ping"
	ping.Ping = true
	ping.Pong = false
	var pong PingPong
	err := ec.Request(topic, ping, &pong, time.Second*15)
	if err != nil {
		return false, err
	}
	return pong.Pong, nil
}

func SPing(ping PingPong) (PingPong, error) {
	ping.Pong = true
	return ping, nil
}
