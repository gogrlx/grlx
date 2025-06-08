package client

import (
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/v2/api"
	"github.com/gogrlx/grlx/v2/auth"
	"github.com/gogrlx/grlx/v2/config"
	"github.com/gogrlx/grlx/v2/types"
)

func GetVersion() (types.Version, error) {
	farmerVersion := types.Version{}
	FarmerURL := config.FarmerURL
	url := FarmerURL + api.Routes["GetVersion"].Pattern // "/test/ping"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return farmerVersion, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return farmerVersion, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return farmerVersion, err
	}
	err = json.NewDecoder(resp.Body).Decode(&farmerVersion)
	return farmerVersion, err
}
