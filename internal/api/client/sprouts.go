package client

import (
	"encoding/json"
	"fmt"
)

// SproutInfo represents a sprout with its key state and connectivity status.
type SproutInfo struct {
	ID        string `json:"id"`
	KeyState  string `json:"key_state"`
	Connected bool   `json:"connected"`
	NKey      string `json:"nkey,omitempty"`
}

// SproutListResponse is the response from the sprouts.list API.
type SproutListResponse struct {
	Sprouts []SproutInfo `json:"sprouts"`
}

// ListSprouts retrieves all sprouts from the farmer.
func ListSprouts() ([]SproutInfo, error) {
	resp, err := NatsRequest("sprouts.list", nil)
	if err != nil {
		return nil, err
	}

	var result SproutListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("list sprouts: %w", err)
	}

	return result.Sprouts, nil
}

// GetSprout retrieves details for a single sprout by ID.
func GetSprout(sproutID string) (*SproutInfo, error) {
	params := map[string]string{"id": sproutID}
	resp, err := NatsRequest("sprouts.get", params)
	if err != nil {
		return nil, err
	}

	var info SproutInfo
	if err := json.Unmarshal(resp, &info); err != nil {
		return nil, fmt.Errorf("get sprout: %w", err)
	}

	return &info, nil
}

// GetSproutProps retrieves all properties for a sprout.
func GetSproutProps(sproutID string) (map[string]interface{}, error) {
	params := map[string]string{"sprout_id": sproutID}
	resp, err := NatsRequest("props.getall", params)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("get sprout props: %w", err)
	}

	return result, nil
}
