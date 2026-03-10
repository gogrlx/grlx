package client

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/pki"
)

func ListKeys() (pki.KeysByType, error) {
	var keys pki.KeysByType
	resp, err := NatsRequest("pki.list", nil)
	if err != nil {
		return keys, err
	}
	if err := json.Unmarshal(resp, &keys); err != nil {
		return keys, fmt.Errorf("list keys: %w", err)
	}
	return keys, nil
}

func AcceptKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []pki.KeySet{
		keyList.Unaccepted,
		keyList.Denied,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
		if keyFound {
			break
		}
	}
	if !keyFound {
		for _, key := range keyList.Accepted.Sprouts {
			if id == key.SproutID {
				return false, pki.ErrAlreadyAccepted
			}
		}
		return false, pki.ErrSproutIDNotFound
	}

	_, err = NatsRequest("pki.accept", pki.KeyManager{SproutID: id})
	if err != nil {
		return false, err
	}
	return true, nil
}

func UnacceptKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []pki.KeySet{
		keyList.Accepted,
		keyList.Denied,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
		if keyFound {
			break
		}
	}
	if !keyFound {
		for _, key := range keyList.Unaccepted.Sprouts {
			if id == key.SproutID {
				return false, pki.ErrAlreadyUnaccepted
			}
		}
		return false, pki.ErrSproutIDNotFound
	}

	_, err = NatsRequest("pki.unaccept", pki.KeyManager{SproutID: id})
	if err != nil {
		return false, err
	}
	return true, nil
}

func RejectKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []pki.KeySet{
		keyList.Accepted,
		keyList.Unaccepted,
		keyList.Denied,
	} {
		for _, key := range keySet.Sprouts {
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
		if keyFound {
			break
		}
	}
	if !keyFound {
		for _, key := range keyList.Rejected.Sprouts {
			if id == key.SproutID {
				return false, pki.ErrAlreadyRejected
			}
		}
		return false, pki.ErrSproutIDNotFound
	}

	_, err = NatsRequest("pki.reject", pki.KeyManager{SproutID: id})
	if err != nil {
		return false, err
	}
	return true, nil
}

func DenyKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []pki.KeySet{
		keyList.Accepted,
		keyList.Unaccepted,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
		if keyFound {
			break
		}
	}
	if !keyFound {
		for _, key := range keyList.Denied.Sprouts {
			if id == key.SproutID {
				return false, pki.ErrAlreadyDenied
			}
		}
		return false, pki.ErrSproutIDNotFound
	}

	_, err = NatsRequest("pki.deny", pki.KeyManager{SproutID: id})
	if err != nil {
		return false, err
	}
	return true, nil
}

func DeleteKey(id string) (bool, error) {
	keyList, err := ListKeys()
	if err != nil {
		return false, err
	}
	keyFound := false
	for _, keySet := range []pki.KeySet{
		keyList.Accepted,
		keyList.Unaccepted,
		keyList.Denied,
		keyList.Rejected,
	} {
		for _, key := range keySet.Sprouts {
			if id == key.SproutID {
				keyFound = true
				break
			}
		}
		if keyFound {
			break
		}
	}
	if !keyFound {
		return false, pki.ErrSproutIDNotFound
	}

	_, err = NatsRequest("pki.delete", pki.KeyManager{SproutID: id})
	if err != nil {
		return false, err
	}
	return true, nil
}
