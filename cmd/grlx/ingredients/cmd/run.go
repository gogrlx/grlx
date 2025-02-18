package cmd

import (
	//. "github.com/gogrlx/grlx/v2/config"
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	pki "github.com/gogrlx/grlx/v2/api/client"
	"github.com/gogrlx/grlx/v2/auth"
	"github.com/gogrlx/grlx/v2/config"
	"github.com/gogrlx/grlx/v2/types"
)

func FRun(target string, command types.CmdRun) (types.TargetedResults, error) {
	// util target split
	// check targets valid
	ctx, cancel := context.WithTimeout(context.Background(), command.Timeout)
	defer cancel()
	var tr types.TargetedResults
	targets, err := pki.ResolveTargets(target)
	if err != nil {
		return tr, err
	}
	var ta types.TargetedAction
	ta.Action = command
	ta.Target = []types.KeyManager{}
	for _, sprout := range targets {
		ta.Target = append(ta.Target, types.KeyManager{SproutID: sprout})
	}
	url := config.FarmerURL + "/cmd/run"
	jw, _ := json.Marshal(ta)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return tr, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return tr, err
	}
	req.Header.Set("Authorization", newToken)
	timeoutClient := &http.Client{}
	timeoutClient.Timeout = command.Timeout
	timeoutClient.Transport = pki.APIClient.Transport
	resp, err := timeoutClient.Do(req)
	if err != nil {
		return tr, err
	}
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
