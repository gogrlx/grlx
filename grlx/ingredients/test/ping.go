package test

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/grlx/util"
	. "github.com/gogrlx/grlx/types"
	"github.com/spf13/viper"
)

func FPing(target string) (TargetedResults, error) {
	FarmerURL := viper.GetString("FarmerURL")
	// util target split
	// check targets valid
	var tr TargetedResults
	targets, err := util.ResolveTargets(target)

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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tr, err
	}
	err = json.NewDecoder(resp.Body).Decode(&tr)
	return tr, err
}
