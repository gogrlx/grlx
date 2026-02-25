package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func Cook(target string, cmdCook apitypes.CmdCook) (apitypes.CmdCook, error) {
	// util target split
	// check targets valid
	client := APIClient
	ctx := context.Background()
	FarmerURL := config.FarmerURL
	targets, err := ResolveTargets(target)
	if err != nil {
		return cmdCook, err
	}
	var ta apitypes.TargetedAction
	ta.Action = cmdCook
	ta.Target = []pki.KeyManager{}
	for _, sprout := range targets {
		ta.Target = append(ta.Target, pki.KeyManager{SproutID: sprout})
	}
	url := FarmerURL + api.Routes["Cook"].Pattern
	jw, _ := json.Marshal(ta)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return cmdCook, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return cmdCook, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := client.Do(req)
	if err != nil {
		return cmdCook, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&cmdCook)
	// TODO connect NATS and start tailing the bus here
	return cmdCook, err
}
