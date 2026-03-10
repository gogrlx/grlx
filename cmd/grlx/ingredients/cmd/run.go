package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	pkiclient "github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func FRun(target string, command apitypes.CmdRun) (apitypes.TargetedResults, error) {
	var tr apitypes.TargetedResults
	targets, err := pkiclient.ResolveTargets(target)
	if err != nil {
		return tr, err
	}

	var ta apitypes.TargetedAction
	ta.Action = command
	ta.Target = make([]pki.KeyManager, len(targets))
	for i, sprout := range targets {
		ta.Target[i] = pki.KeyManager{SproutID: sprout}
	}

	// Use the command timeout for the NATS request if set.
	if command.Timeout > 0 {
		origTimeout := pkiclient.NatsRequestTimeout
		pkiclient.NatsRequestTimeout = command.Timeout + 5*time.Second
		defer func() { pkiclient.NatsRequestTimeout = origTimeout }()
	}

	resp, err := pkiclient.NatsRequest("cmd.run", ta)
	if err != nil {
		return tr, err
	}
	if err := json.Unmarshal(resp, &tr); err != nil {
		return tr, fmt.Errorf("cmd.run: %w", err)
	}
	return tr, nil
}
