package test

import (
	"time"

	. "github.com/gogrlx/grlx/types"
)

// TODO allow selector to be more than an ID
func FPing(id string) (bool, error) {
	topic := "grlx.sprouts." + id + ".test.ping"
	var ping PingPong
	ping.Ping = time.Now()
	var pong PingPong
	err := ec.Request(topic, ping, &pong, time.Second*15)
	if err != nil {
		return false, err
	}
	if pong.Pong.After(ping.Pong) {
		return true, nil
	}
	return false, nil
}

func SPing(ping PingPong) (PingPong, error) {
	ping.Pong = time.Now()
	return ping, nil
}
