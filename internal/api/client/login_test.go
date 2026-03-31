package client

import (
	"encoding/json"
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
)

func TestLoginResponseUnmarshal(t *testing.T) {
	data := `{
		"authenticated": true,
		"pubkey": "AXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
		"role": "admin",
		"username": "alice",
		"isAdmin": true,
		"actions": [{"action": "admin", "scope": "*"}],
		"message": "authenticated as alice (role: admin)"
	}`

	var resp apitypes.LoginResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Authenticated {
		t.Error("expected authenticated=true")
	}
	if resp.RoleName != "admin" {
		t.Errorf("expected role=admin, got %s", resp.RoleName)
	}
	if resp.Username != "alice" {
		t.Errorf("expected username=alice, got %s", resp.Username)
	}
	if !resp.IsAdmin {
		t.Error("expected isAdmin=true")
	}
	if len(resp.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(resp.Actions))
	}
}

func TestLoginResponseUnauthenticated(t *testing.T) {
	data := `{
		"authenticated": false,
		"message": "no token provided"
	}`

	var resp apitypes.LoginResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Authenticated {
		t.Error("expected authenticated=false")
	}
	if resp.Message != "no token provided" {
		t.Errorf("expected message='no token provided', got %q", resp.Message)
	}
}
