package cmd

import (
	//. "github.com/gogrlx/grlx/config"
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	pki "github.com/gogrlx/grlx/api/client"
	"github.com/gogrlx/grlx/auth"
	"github.com/gogrlx/grlx/config"
	. "github.com/gogrlx/grlx/types"
)

func FRun(target string, command CmdRun) (TargetedResults, error) {
	// util target split
	// check targets valid
	ctx, cancel := context.WithTimeout(context.Background(), command.Timeout)
	defer cancel()
	var tr TargetedResults
	FarmerURL := config.FarmerURL
	targets, err := pki.ResolveTargets(target)
	if err != nil {
		return tr, err
	}
	var ta TargetedAction
	ta.Action = command
	ta.Target = []KeyManager{}
	for _, sprout := range targets {
		ta.Target = append(ta.Target, KeyManager{SproutID: sprout})
	}
	url := FarmerURL + "/cmd/run"
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
	resp, err := pki.APIClient.Do(req)
	if err != nil {
		return tr, err
	}
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
