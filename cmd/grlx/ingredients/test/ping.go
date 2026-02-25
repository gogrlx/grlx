package test

import (
	"bytes"
	"encoding/json"
	"net/http"

	pkiclient "github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func FPing(target string) (apitypes.TargetedResults, error) {
	FarmerURL := config.FarmerURL
	// util target split
	// check targets valid
	var tr apitypes.TargetedResults
	targets, err := pkiclient.ResolveTargets(target)
	if err != nil {
		return tr, err
	}
	var ta apitypes.TargetedAction
	ta.Action = apitypes.PingPong{}
	ta.Target = []pki.KeyManager{}
	for _, sprout := range targets {
		ta.Target = append(ta.Target, pki.KeyManager{SproutID: sprout})
	}
	url := FarmerURL + "/test/ping"
	jw, _ := json.Marshal(ta)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
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
	resp, err := pkiclient.APIClient.Do(req)
	if err != nil {
		return tr, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
