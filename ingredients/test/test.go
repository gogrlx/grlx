package test

import nats "github.com/nats-io/nats.go"

var nc *nats.Conn

func RegisterNatsConn(n *nats.Conn) {
	nc = n
}
