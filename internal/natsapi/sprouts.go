package natsapi

import (
	"encoding/json"
	"fmt"
	"time"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

const sproutPingTimeout = 3 * time.Second

// SproutInfo represents a sprout with its key state and connectivity status.
type SproutInfo struct {
	ID        string `json:"id"`
	KeyState  string `json:"key_state"`
	Connected bool   `json:"connected"`
	NKey      string `json:"nkey,omitempty"`
}

func handleSproutsList(_ json.RawMessage) (any, error) {
	allKeys := pki.ListNKeysByType()
	var sprouts []SproutInfo

	type entry struct {
		id    string
		state string
	}
	var entries []entry
	for _, km := range allKeys.Accepted.Sprouts {
		entries = append(entries, entry{id: km.SproutID, state: "accepted"})
	}
	for _, km := range allKeys.Unaccepted.Sprouts {
		entries = append(entries, entry{id: km.SproutID, state: "unaccepted"})
	}
	for _, km := range allKeys.Denied.Sprouts {
		entries = append(entries, entry{id: km.SproutID, state: "denied"})
	}
	for _, km := range allKeys.Rejected.Sprouts {
		entries = append(entries, entry{id: km.SproutID, state: "rejected"})
	}

	for _, e := range entries {
		info := SproutInfo{
			ID:       e.id,
			KeyState: e.state,
		}
		nkey, err := pki.GetNKey(e.id)
		if err == nil {
			info.NKey = nkey
		}
		if e.state == "accepted" && natsConn != nil {
			info.Connected = probeSprout(e.id)
		}
		sprouts = append(sprouts, info)
	}

	if sprouts == nil {
		sprouts = []SproutInfo{}
	}
	return map[string][]SproutInfo{"sprouts": sprouts}, nil
}

func handleSproutsGet(params json.RawMessage) (any, error) {
	var km pki.KeyManager
	if err := json.Unmarshal(params, &km); err != nil {
		return nil, err
	}
	if !pki.IsValidSproutID(km.SproutID) {
		return nil, fmt.Errorf("invalid sprout ID")
	}

	nkey, err := pki.GetNKey(km.SproutID)
	if err != nil {
		return nil, fmt.Errorf("sprout not found")
	}

	keyState := resolveKeyState(km.SproutID)
	info := SproutInfo{
		ID:       km.SproutID,
		KeyState: keyState,
		NKey:     nkey,
	}

	if keyState == "accepted" && natsConn != nil {
		info.Connected = probeSprout(km.SproutID)
	}

	return info, nil
}

func probeSprout(sproutID string) bool {
	if natsConn == nil {
		return false
	}
	topic := "grlx.sprouts." + sproutID + ".test.ping"
	ping := apitypes.PingPong{Ping: true}
	data, err := json.Marshal(ping)
	if err != nil {
		return false
	}
	msg, err := natsConn.Request(topic, data, sproutPingTimeout)
	if err != nil {
		return false
	}
	var pong apitypes.PingPong
	if err := json.Unmarshal(msg.Data, &pong); err != nil {
		return false
	}
	return pong.Pong
}

func resolveKeyState(sproutID string) string {
	allKeys := pki.ListNKeysByType()
	for _, km := range allKeys.Accepted.Sprouts {
		if km.SproutID == sproutID {
			return "accepted"
		}
	}
	for _, km := range allKeys.Unaccepted.Sprouts {
		if km.SproutID == sproutID {
			return "unaccepted"
		}
	}
	for _, km := range allKeys.Denied.Sprouts {
		if km.SproutID == sproutID {
			return "denied"
		}
	}
	for _, km := range allKeys.Rejected.Sprouts {
		if km.SproutID == sproutID {
			return "rejected"
		}
	}
	return "unknown"
}
