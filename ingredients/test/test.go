package test

import nats "github.com/nats-io/nats.go"

var ec *nats.EncodedConn

func RegisterEC(encodedConn *nats.EncodedConn) {
	ec = encodedConn
}
