package client

import (
	"encoding/json"
	"testing"
)

func TestSproutInfoUnmarshal(t *testing.T) {
	raw := `{"id":"web-01","key_state":"accepted","connected":true,"nkey":"NFOO"}`
	var info SproutInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.ID != "web-01" {
		t.Fatalf("expected ID web-01, got %q", info.ID)
	}
	if info.KeyState != "accepted" {
		t.Fatalf("expected KeyState accepted, got %q", info.KeyState)
	}
	if !info.Connected {
		t.Fatal("expected Connected true")
	}
	if info.NKey != "NFOO" {
		t.Fatalf("expected NKey NFOO, got %q", info.NKey)
	}
}

func TestSproutListResponseUnmarshal(t *testing.T) {
	raw := `{"sprouts":[{"id":"a","key_state":"accepted","connected":true},{"id":"b","key_state":"unaccepted","connected":false}]}`
	var resp SproutListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Sprouts) != 2 {
		t.Fatalf("expected 2 sprouts, got %d", len(resp.Sprouts))
	}
	if resp.Sprouts[0].ID != "a" {
		t.Fatalf("expected first sprout ID a, got %q", resp.Sprouts[0].ID)
	}
	if resp.Sprouts[1].KeyState != "unaccepted" {
		t.Fatalf("expected second sprout state unaccepted, got %q", resp.Sprouts[1].KeyState)
	}
}

func TestSproutListResponseEmpty(t *testing.T) {
	raw := `{"sprouts":[]}`
	var resp SproutListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Sprouts) != 0 {
		t.Fatalf("expected 0 sprouts, got %d", len(resp.Sprouts))
	}
}

func TestSproutInfoMarshal(t *testing.T) {
	info := SproutInfo{
		ID:        "db-01",
		KeyState:  "accepted",
		Connected: false,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["id"] != "db-01" {
		t.Fatalf("expected id db-01, got %v", decoded["id"])
	}
	// nkey should be omitted when empty
	if _, ok := decoded["nkey"]; ok {
		t.Fatal("expected nkey to be omitted when empty")
	}
}
