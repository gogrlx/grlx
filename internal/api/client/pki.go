package client

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/types"
)

func ListKeys() (types.KeysByType, error) {
	var keys types.KeysByType
	FarmerURL := config.FarmerURL
	url := FarmerURL + api.Routes["ListID"].Pattern
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return keys, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return keys, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return keys, err
	}
	err = json.NewDecoder(resp.Body).Decode(&keys)
	return keys, err
}

func UnacceptKey(id string) (bool, error) {
	keyList, err := ListKeys()
	FarmerURL := config.FarmerURL
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []types.KeySet{
		keyList.Accepted,
		keyList.Denied,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if keyFound {
				break
			}
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
	}
	if !keyFound {
		for _, key := range keyList.Unaccepted.Sprouts {
			if id == key.SproutID {
				return false, types.ErrAlreadyUnaccepted
			}
		}
		return false, types.ErrSproutIDNotFound
	}
	var success types.Inline
	url := FarmerURL + api.Routes["UnacceptID"].Pattern
	km := types.KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return false, err
	}
	err = json.NewDecoder(resp.Body).Decode(&success)
	if err != nil {
		return false, err
	}
	return success.Success, success.Error
}

func DenyKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []types.KeySet{
		keyList.Accepted,
		keyList.Unaccepted,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if keyFound {
				break
			}
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
	}
	if !keyFound {
		for _, key := range keyList.Denied.Sprouts {
			if id == key.SproutID {
				return false, types.ErrAlreadyDenied
			}
		}

		return false, types.ErrSproutIDNotFound
	}
	var success types.Inline
	FarmerURL := config.FarmerURL
	url := FarmerURL + api.Routes["DenyID"].Pattern
	km := types.KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return false, err
	}
	err = json.NewDecoder(resp.Body).Decode(&success)
	if err != nil {
		return false, err
	}
	return success.Success, success.Error
}

func RejectKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []types.KeySet{
		keyList.Accepted,
		keyList.Unaccepted,
		keyList.Denied,
	} {
		for _, key := range keySet.Sprouts {
			if keyFound {
				break
			}
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
	}
	if !keyFound {
		for _, key := range keyList.Rejected.Sprouts {
			if id == key.SproutID {
				return false, types.ErrAlreadyRejected
			}
		}
		return false, types.ErrSproutIDNotFound
	}
	var success types.Inline
	FarmerURL := config.FarmerURL
	url := FarmerURL + "/pki/rejectnkey"
	km := types.KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return false, err
	}
	err = json.NewDecoder(resp.Body).Decode(&success)
	if err != nil {
		return false, err
	}
	return success.Success, success.Error
}

func DeleteKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []types.KeySet{
		keyList.Accepted,
		keyList.Unaccepted,
		keyList.Denied,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if keyFound {
				break
			}
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
	}
	if !keyFound {
		return false, types.ErrSproutIDNotFound
	}
	var success types.Inline
	FarmerURL := config.FarmerURL
	url := FarmerURL + "/pki/deletenkey"
	km := types.KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return false, err
	}
	err = json.NewDecoder(resp.Body).Decode(&success)
	if err != nil {
		return false, err
	}
	return success.Success, success.Error
}

func AcceptKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []types.KeySet{
		keyList.Unaccepted,
		keyList.Denied,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if keyFound {
				break
			}
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
	}
	if !keyFound {
		for _, key := range keyList.Accepted.Sprouts {
			if id == key.SproutID {
				return false, types.ErrAlreadyAccepted
			}
		}
		return false, types.ErrSproutIDNotFound
	}
	var success types.Inline
	FarmerURL := config.FarmerURL
	url := FarmerURL + api.Routes["AcceptID"].Pattern
	km := types.KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	newToken, err := auth.NewToken()
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", newToken)
	resp, err := APIClient.Do(req)
	if err != nil {
		return false, err
	}
	err = json.NewDecoder(resp.Body).Decode(&success)
	if err != nil {
		return false, err
	}
	return success.Success, success.Error
}
