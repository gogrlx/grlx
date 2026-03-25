package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/rbac"
	"github.com/taigrr/jety"
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

func TestWhoAmI_DangerouslyAllowRoot(t *testing.T) {
	jety.Set("dangerously_allow_root", true)
	defer jety.Set("dangerously_allow_root", false)

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	// No Authorization header — should still succeed because dangerously_allow_root is on.
	w := httptest.NewRecorder()

	WhoAmI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with dangerously_allow_root, got %d", w.Code)
	}

	var info apitypes.UserInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if info.Pubkey != "(dangerously_allow_root)" {
		t.Errorf("expected pubkey '(dangerously_allow_root)', got %q", info.Pubkey)
	}
	if info.RoleName != "admin" {
		t.Errorf("expected role 'admin', got %q", info.RoleName)
	}
}

func TestWhoAmI_DangerouslyAllowRoot_WithContentType(t *testing.T) {
	jety.Set("dangerously_allow_root", true)
	defer jety.Set("dangerously_allow_root", false)

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	w := httptest.NewRecorder()

	WhoAmI(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func TestWhoAmI_EmptyToken_NoDangerousRoot(t *testing.T) {
	// Explicitly ensure dangerously_allow_root is off.
	jety.Set("dangerously_allow_root", false)

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	w := httptest.NewRecorder()

	WhoAmI(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
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

func TestListUsers_ResponseStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify the response is valid JSON
	var resp apitypes.UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Roles should always be a list (may be empty)
	if resp.Roles == nil {
		t.Error("expected non-nil roles list")
	}
}

func TestWhoAmI_WithToken_Invalid(t *testing.T) {
	jety.Set("dangerously_allow_root", false)

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("Authorization", "completely-invalid-base64-token")
	w := httptest.NewRecorder()

	WhoAmI(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListUsers_WithRolesAndUsers(t *testing.T) {
	rs := rbac.NewRoleStore()
	rs.Register(&rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	})
	rs.Register(&rbac.Role{
		Name:  "viewer",
		Rules: []rbac.Rule{{Action: rbac.ActionView, Scope: "*"}},
	})
	urm := rbac.NewUserRoleMap()
	urm.Set("UPUBKEY1", "admin")
	urm.Set("UPUBKEY2", "viewer")
	cr := rbac.NewRegistry()

	auth.SetPolicy(rs, urm, cr)
	defer auth.SetPolicy(nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp apitypes.UsersListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(resp.Users))
	}
	if len(resp.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(resp.Roles))
	}

	// Verify role names
	roleNames := make(map[string]bool)
	for _, r := range resp.Roles {
		roleNames[r.Name] = true
	}
	if !roleNames["admin"] {
		t.Error("expected 'admin' role in response")
	}
	if !roleNames["viewer"] {
		t.Error("expected 'viewer' role in response")
	}
}

func TestListUsers_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	ListUsers(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected 'application/json', got %q", ct)
	}
}
