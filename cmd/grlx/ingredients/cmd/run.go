package cmd

import (
	//. "github.com/gogrlx/grlx/v2/internal/config"
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	pkiclient "github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func FRun(target string, command apitypes.CmdRun) (apitypes.TargetedResults, error) {
	// util target split
	// check targets valid
	ctx, cancel := context.WithTimeout(context.Background(), command.Timeout)
	defer cancel()
	var tr apitypes.TargetedResults
	targets, err := pkiclient.ResolveTargets(target)
	if err != nil {
		return tr, err
	}
	var ta apitypes.TargetedAction
	ta.Action = command
	ta.Target = []pki.KeyManager{}
	for _, sprout := range targets {
		ta.Target = append(ta.Target, pki.KeyManager{SproutID: sprout})
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
	timeoutClient.Transport = pkiclient.APIClient.Transport
	resp, err := timeoutClient.Do(req)
	if err != nil {
		return tr, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
