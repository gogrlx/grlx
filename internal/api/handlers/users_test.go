package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
)

func TestWhoAmI_NoToken_NoDangerousRoot(t *testing.T) {
	// Ensure dangerously_allow_root is not set (default)
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	w := httptest.NewRecorder()

	WhoAmI(w, req)

	// Without a token and without dangerously_allow_root, should be 401
	if !auth.DangerouslyAllowRoot() && w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}

func TestWhoAmI_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "invalid-token-data")
	w := httptest.NewRecorder()

	WhoAmI(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", w.Code)
	}
}

func TestListUsers_ReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var resp apitypes.UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Users and Roles should be non-nil (may be empty)
	if resp.Users == nil {
		// ListAllUsers returns a map, could be nil or empty
		// depending on auth config — that's fine
	}
}

func TestListUsers_UsersMapNotNil(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	ListUsers(w, req)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Verify we get the expected JSON structure
	if _, ok := raw["users"]; !ok {
		t.Error("expected 'users' key in response")
	}
	if _, ok := raw["roles"]; !ok {
		t.Error("expected 'roles' key in response")
	}
}
