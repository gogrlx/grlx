//go:build windows

package shell

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// SproutSession is a placeholder on Windows where PTY support is not available.
type SproutSession struct{}

// HandleShellStart returns an error on Windows since PTY is not supported.
func HandleShellStart(nc *nats.Conn, msg *nats.Msg) {
	resp := struct {
		Error string `json:"error"`
	}{Error: "interactive shell is not supported on Windows sprouts"}
	data, _ := json.Marshal(resp)
	msg.Respond(data)
}

func respondError(msg *nats.Msg, err error) {
	resp := struct {
		Error string `json:"error"`
	}{Error: err.Error()}
	data, _ := json.Marshal(resp)
	msg.Respond(data)
}
