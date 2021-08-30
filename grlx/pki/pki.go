package pki

import (
	"bytes"
	"encoding/json"
	"net/http"

	. "github.com/gogrlx/grlx/types"
	"github.com/spf13/viper"
)

func ListKeys() (KeysByType, error) {
	var keys KeysByType
	FarmerURL := viper.GetString("FarmerURL")
	url := FarmerURL + "/pki/listnkey"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return keys, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return keys, err
	}
	err = json.NewDecoder(resp.Body).Decode(&keys)
	return keys, err
}
func UnacceptKey(id string) (bool, error) {
	keyList, err := ListKeys()
	FarmerURL := viper.GetString("FarmerURL")
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []KeySet{keyList.Accepted,
		keyList.Denied,
		keyList.Rejected} {
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
				return false, ErrAlreadyUnaccepted
			}
		}

		return false, ErrSproutIDNotFound
	}
	var success Inline
	url := FarmerURL + "/pki/unacceptnkey"
	km := KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
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
	for _, keySet := range []KeySet{keyList.Accepted,
		keyList.Unaccepted,
		keyList.Rejected} {
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
				return false, ErrAlreadyDenied
			}
		}

		return false, ErrSproutIDNotFound
	}
	var success Inline
	FarmerURL := viper.GetString("FarmerURL")
	url := FarmerURL + "/pki/denynkey"
	km := KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
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
	for _, keySet := range []KeySet{keyList.Accepted,
		keyList.Unaccepted,
		keyList.Denied} {
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
				return false, ErrAlreadyRejected
			}
		}

		return false, ErrSproutIDNotFound
	}
	var success Inline
	FarmerURL := viper.GetString("FarmerURL")
	url := FarmerURL + "/pki/rejectnkey"
	km := KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
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
	for _, keySet := range []KeySet{keyList.Accepted,
		keyList.Unaccepted,
		keyList.Denied,
		keyList.Rejected} {
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
		return false, ErrSproutIDNotFound
	}
	var success Inline
	FarmerURL := viper.GetString("FarmerURL")
	url := FarmerURL + "/pki/deletenkey"
	km := KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
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
	for _, keySet := range []KeySet{keyList.Unaccepted,
		keyList.Denied,
		keyList.Rejected} {
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
				return false, ErrAlreadyAccepted
			}
		}
		return false, ErrSproutIDNotFound
	}
	var success Inline
	FarmerURL := viper.GetString("FarmerURL")
	url := FarmerURL + "/pki/acceptnkey"
	km := KeyManager{SproutID: id}
	jw, _ := json.Marshal(km)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jw))
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	err = json.NewDecoder(resp.Body).Decode(&success)
	if err != nil {
		return false, err
	}
	return success.Success, success.Error
}
