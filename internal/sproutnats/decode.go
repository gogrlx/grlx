package sproutnats

import (
	"encoding/json"
	"fmt"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/cook"
)

// DecodeCmdRun parses a sprout command request and returns a response-safe
// failed command result when the payload is malformed.
func DecodeCmdRun(data []byte) (apitypes.CmdRun, apitypes.CmdRun, error) {
	var cmdRun apitypes.CmdRun
	if err := json.Unmarshal(data, &cmdRun); err != nil {
		return apitypes.CmdRun{}, apitypes.CmdRun{
			Stderr:  fmt.Sprintf("invalid cmd.run request: %v", err),
			ErrCode: -1,
		}, err
	}
	return cmdRun, apitypes.CmdRun{}, nil
}

// DecodePing parses a sprout ping request and returns a negative pong response
// when the payload is malformed.
func DecodePing(data []byte) (apitypes.PingPong, apitypes.PingPong, error) {
	var ping apitypes.PingPong
	if err := json.Unmarshal(data, &ping); err != nil {
		return apitypes.PingPong{}, apitypes.PingPong{Ping: false, Pong: false}, err
	}
	return ping, apitypes.PingPong{}, nil
}

// DecodeCook parses a sprout cook request and returns a negative ack when the
// payload is malformed.
func DecodeCook(data []byte) (cook.RecipeEnvelope, cook.Ack, error) {
	var envelope cook.RecipeEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return cook.RecipeEnvelope{}, cook.Ack{Acknowledged: false}, err
	}
	return envelope, cook.Ack{}, nil
}
