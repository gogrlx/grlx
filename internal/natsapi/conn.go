package natsapi

import "github.com/nats-io/nats.go"

// natsConn is the NATS connection used by handlers that need to
// communicate with sprouts (cook, cmd, test, cancel, probe).
var natsConn *nats.Conn

// SetNatsConn sets the NATS connection for API handlers.
func SetNatsConn(nc *nats.Conn) {
	natsConn = nc
}
