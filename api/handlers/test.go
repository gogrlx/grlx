package handlers

import nats "github.com/nats-io/nats.go"

var conn *nats.Conn

func RegisterNatsConn(n *nats.Conn) {
	conn = n
}
