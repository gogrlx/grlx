package handlers

import nats "github.com/nats-io/nats.go"

var ec *nats.EncodedConn

func RegisterEC(n *nats.EncodedConn) {
	ec = n
}
