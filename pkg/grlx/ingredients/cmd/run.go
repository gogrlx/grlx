package cmd

import (
	//. "github.com/gogrlx/grlx/config"
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/auth"
	"github.com/gogrlx/grlx/pkg/grlx/util"
	. "github.com/gogrlx/grlx/types"
	"github.com/spf13/viper"
)

func FRun(target string, command CmdRun) (TargetedResults, error) {
	// util target split
	// check targets valid
	client := http.Client{}
	client.Timeout = command.Timeout
	ctx, cancel := context.WithTimeout(context.Background(), command.Timeout)
	defer cancel()
	var tr TargetedResults
	FarmerURL := viper.GetString("FarmerURL")
	targets, err := util.ResolveTargets(target)
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
	resp, err := client.Do(req)
	if err != nil {
		return tr, err
	}
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
