package test

import (
	"encoding/json"
	"fmt"

	pkiclient "github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func FPing(target string) (apitypes.TargetedResults, error) {
	var tr apitypes.TargetedResults
	targets, err := pkiclient.ResolveTargets(target)
	if err != nil {
		return tr, err
	}

	var ta apitypes.TargetedAction
	ta.Action = apitypes.PingPong{}
	ta.Target = make([]pki.KeyManager, len(targets))
	for i, sprout := range targets {
		ta.Target[i] = pki.KeyManager{SproutID: sprout}
	}

	resp, err := pkiclient.NatsRequest("test.ping", ta)
	if err != nil {
		return tr, err
	}
	if err := json.Unmarshal(resp, &tr); err != nil {
		return tr, fmt.Errorf("test.ping: %w", err)
	}
	return tr, nil
}
