package client

import (
	"testing"
)

func TestListSprouts_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := SproutListResponse{
		Sprouts: []SproutInfo{
			{ID: "web-01", KeyState: "accepted", Connected: true},
			{ID: "web-02", KeyState: "accepted", Connected: false},
			{ID: "db-01", KeyState: "unaccepted", Connected: false},
		},
	}
	mockHandler(t, NatsConn, "grlx.api.sprouts.list", want)

	got, err := ListSprouts()
	if err != nil {
		t.Fatalf("ListSprouts: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 sprouts, got %d", len(got))
	}
	if got[0].ID != "web-01" {
		t.Fatalf("expected web-01, got %q", got[0].ID)
	}
	if !got[0].Connected {
		t.Fatal("expected web-01 connected")
	}
	if got[2].KeyState != "unaccepted" {
		t.Fatalf("expected unaccepted, got %q", got[2].KeyState)
	}
}

func TestListSprouts_Empty(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := SproutListResponse{Sprouts: []SproutInfo{}}
	mockHandler(t, NatsConn, "grlx.api.sprouts.list", want)

	got, err := ListSprouts()
	if err != nil {
		t.Fatalf("ListSprouts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 sprouts, got %d", len(got))
	}
}

func TestListSprouts_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.sprouts.list", "connection refused")

	_, err := ListSprouts()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSprout_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := SproutInfo{
		ID:        "web-01",
		KeyState:  "accepted",
		Connected: true,
		NKey:      "NFOO123",
	}
	mockHandler(t, NatsConn, "grlx.api.sprouts.get", want)

	got, err := GetSprout("web-01")
	if err != nil {
		t.Fatalf("GetSprout: %v", err)
	}
	if got.ID != "web-01" {
		t.Fatalf("expected web-01, got %q", got.ID)
	}
	if got.NKey != "NFOO123" {
		t.Fatalf("expected NKey NFOO123, got %q", got.NKey)
	}
}

func TestGetSprout_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.sprouts.get", "sprout not found")

	_, err := GetSprout("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSproutProps_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := map[string]interface{}{
		"os":       "linux",
		"arch":     "amd64",
		"hostname": "web-01.example.com",
	}
	mockHandler(t, NatsConn, "grlx.api.props.getall", want)

	got, err := GetSproutProps("web-01")
	if err != nil {
		t.Fatalf("GetSproutProps: %v", err)
	}
	if got["os"] != "linux" {
		t.Fatalf("expected os linux, got %v", got["os"])
	}
	if got["hostname"] != "web-01.example.com" {
		t.Fatalf("expected hostname web-01.example.com, got %v", got["hostname"])
	}
}

func TestGetSproutProps_Empty(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockHandler(t, NatsConn, "grlx.api.props.getall", map[string]interface{}{})

	got, err := GetSproutProps("new-sprout")
	if err != nil {
		t.Fatalf("GetSproutProps: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty props, got %d", len(got))
	}
}

func TestGetSproutProps_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.props.getall", "sprout offline")

	_, err := GetSproutProps("offline-sprout")
	if err == nil {
		t.Fatal("expected error")
	}
}
