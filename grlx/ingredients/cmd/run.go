package cmd

import (
	//. "github.com/gogrlx/grlx/config"
	"bytes"
	"encoding/json"
	"net/http"

	. "github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/grlx/util"
	. "github.com/gogrlx/grlx/types"
)

func FRun(target string, command CmdRun) (TargetedResults, error) {
	// util target split
	// check targets valid
	var tr TargetedResults
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
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return tr, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tr, err
	}
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
