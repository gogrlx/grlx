package client

import (
	"encoding/json"
	"fmt"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func Cook(target string, cmdCook apitypes.CmdCook) (apitypes.CmdCook, error) {
	targets, err := ResolveTargets(target)
	if err != nil {
		return cmdCook, err
	}

	var ta apitypes.TargetedAction
	ta.Action = cmdCook
	ta.Target = make([]pki.KeyManager, len(targets))
	for i, sprout := range targets {
		ta.Target[i] = pki.KeyManager{SproutID: sprout}
	}

	resp, err := NatsRequest("cook", ta)
	if err != nil {
		return cmdCook, err
	}
	if err := json.Unmarshal(resp, &cmdCook); err != nil {
		return cmdCook, fmt.Errorf("cook: %w", err)
	}
	return cmdCook, nil
}
