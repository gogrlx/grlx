package test

import (
	"bytes"
	"encoding/json"
	"net/http"

	pki "github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	. "github.com/gogrlx/grlx/v2/types"
)

func FPing(target string) (TargetedResults, error) {
	FarmerURL := config.FarmerURL
	// util target split
	// check targets valid
	var tr TargetedResults
	targets, err := pki.ResolveTargets(target)
	if err != nil {
		return tr, err
	}
	var ta TargetedAction
	ta.Action = PingPong{}
	ta.Target = []KeyManager{}
	for _, sprout := range targets {
		ta.Target = append(ta.Target, KeyManager{SproutID: sprout})
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
	resp, err := pki.APIClient.Do(req)
	if err != nil {
		return tr, err
	}
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
