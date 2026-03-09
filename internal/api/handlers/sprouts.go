package handlers

import (
	"encoding/json"
	"net/http"
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

// SproutListResponse is the response for the GET /sprouts endpoint.
type SproutListResponse struct {
	Sprouts []SproutInfo `json:"sprouts"`
}

// ListSprouts returns all known sprouts with their key state and connectivity.
// Only accepted sprouts are probed for connectivity via NATS ping.
func ListSprouts(w http.ResponseWriter, _ *http.Request) {
	allKeys := pki.ListNKeysByType()
	var sprouts []SproutInfo

	// Collect sprouts from each key state
	type sproutEntry struct {
		id    string
		state string
	}
	var entries []sproutEntry
	for _, km := range allKeys.Accepted.Sprouts {
		entries = append(entries, sproutEntry{id: km.SproutID, state: "accepted"})
	}
	for _, km := range allKeys.Unaccepted.Sprouts {
		entries = append(entries, sproutEntry{id: km.SproutID, state: "unaccepted"})
	}
	for _, km := range allKeys.Denied.Sprouts {
		entries = append(entries, sproutEntry{id: km.SproutID, state: "denied"})
	}
	for _, km := range allKeys.Rejected.Sprouts {
		entries = append(entries, sproutEntry{id: km.SproutID, state: "rejected"})
	}

	// For accepted sprouts, check connectivity via NATS ping
	for _, entry := range entries {
		info := SproutInfo{
			ID:       entry.id,
			KeyState: entry.state,
		}
		nkey, err := pki.GetNKey(entry.id)
		if err == nil {
			info.NKey = nkey
		}
		if entry.state == "accepted" && conn != nil {
			info.Connected = probeSprout(entry.id)
		}
		sprouts = append(sprouts, info)
	}

	if sprouts == nil {
		sprouts = []SproutInfo{}
	}

	resp := SproutListResponse{Sprouts: sprouts}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetSprout returns information about a single sprout by ID.
func GetSprout(w http.ResponseWriter, r *http.Request) {
	var km pki.KeyManager
	err := json.NewDecoder(r.Body).Decode(&km)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !pki.IsValidSproutID(km.SproutID) {
		w.WriteHeader(http.StatusBadRequest)
		jw, _ := json.Marshal(apitypes.Inline{Success: false, Error: pki.ErrSproutIDInvalid})
		w.Write(jw)
		return
	}

	nkey, err := pki.GetNKey(km.SproutID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		jw, _ := json.Marshal(apitypes.Inline{Success: false, Error: err})
		w.Write(jw)
		return
	}

	// Determine key state by checking each category
	keyState := resolveKeyState(km.SproutID)

	info := SproutInfo{
		ID:       km.SproutID,
		KeyState: keyState,
		NKey:     nkey,
	}

	if keyState == "accepted" && conn != nil {
		info.Connected = probeSprout(km.SproutID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

// probeSprout sends a NATS ping to a sprout and returns true if it responds.
func probeSprout(sproutID string) bool {
	topic := "grlx.sprouts." + sproutID + ".test.ping"
	ping := apitypes.PingPong{Ping: true}
	data, err := json.Marshal(ping)
	if err != nil {
		return false
	}
	msg, err := conn.Request(topic, data, sproutPingTimeout)
	if err != nil {
		return false
	}
	var pong apitypes.PingPong
	if err := json.Unmarshal(msg.Data, &pong); err != nil {
		return false
	}
	return pong.Pong
}

// resolveKeyState determines which key state directory contains the sprout.
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
