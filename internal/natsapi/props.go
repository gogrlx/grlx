package natsapi

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/props"
)

// PropsParams holds the sprout and property identifiers.
type PropsParams struct {
	SproutID string `json:"sprout_id"`
	Name     string `json:"name,omitempty"`
	Value    string `json:"value,omitempty"`
}

func handlePropsGetAll(params json.RawMessage) (any, error) {
	var p PropsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SproutID == "" {
		return nil, fmt.Errorf("sprout_id is required")
	}
	allProps := props.GetProps(p.SproutID)
	if allProps == nil {
		allProps = make(map[string]interface{})
	}
	return allProps, nil
}

func handlePropsGet(params json.RawMessage) (any, error) {
	var p PropsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SproutID == "" || p.Name == "" {
		return nil, fmt.Errorf("sprout_id and name are required")
	}
	value := props.GetStringProp(p.SproutID, p.Name)
	return map[string]string{
		"sprout_id": p.SproutID,
		"name":      p.Name,
		"value":     value,
	}, nil
}

func handlePropsSet(params json.RawMessage) (any, error) {
	var p PropsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SproutID == "" || p.Name == "" {
		return nil, fmt.Errorf("sprout_id and name are required")
	}
	if err := props.SetProp(p.SproutID, p.Name, p.Value); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func handlePropsDelete(params json.RawMessage) (any, error) {
	var p PropsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SproutID == "" || p.Name == "" {
		return nil, fmt.Errorf("sprout_id and name are required")
	}
	if err := props.DeleteProp(p.SproutID, p.Name); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
