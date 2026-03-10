package natsapi

import (
	"encoding/json"

	"github.com/gogrlx/grlx/v2/internal/pki"
)

func handlePKIList(_ json.RawMessage) (any, error) {
	return pki.ListNKeysByType(), nil
}

func handlePKIAccept(params json.RawMessage) (any, error) {
	var km pki.KeyManager
	if err := json.Unmarshal(params, &km); err != nil {
		return nil, err
	}
	err := pki.AcceptNKey(km.SproutID)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func handlePKIReject(params json.RawMessage) (any, error) {
	var km pki.KeyManager
	if err := json.Unmarshal(params, &km); err != nil {
		return nil, err
	}
	err := pki.RejectNKey(km.SproutID, "")
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func handlePKIDeny(params json.RawMessage) (any, error) {
	var km pki.KeyManager
	if err := json.Unmarshal(params, &km); err != nil {
		return nil, err
	}
	err := pki.DenyNKey(km.SproutID)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func handlePKIUnaccept(params json.RawMessage) (any, error) {
	var km pki.KeyManager
	if err := json.Unmarshal(params, &km); err != nil {
		return nil, err
	}
	err := pki.UnacceptNKey(km.SproutID, "")
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func handlePKIDelete(params json.RawMessage) (any, error) {
	var km pki.KeyManager
	if err := json.Unmarshal(params, &km); err != nil {
		return nil, err
	}
	err := pki.DeleteNKey(km.SproutID)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
