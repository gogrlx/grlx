package client

import (
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

func TestWhoAmI_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := apitypes.UserInfo{
		Pubkey:   "NKEY123ABC",
		RoleName: "admin",
	}
	mockHandler(t, NatsConn, "grlx.api.auth.whoami", want)

	got, err := WhoAmI()
	if err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if got.Pubkey != "NKEY123ABC" {
		t.Fatalf("expected pubkey NKEY123ABC, got %q", got.Pubkey)
	}
	if got.RoleName != "admin" {
		t.Fatalf("expected role admin, got %q", got.RoleName)
	}
}

func TestWhoAmI_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.auth.whoami", "unauthenticated")

	_, err := WhoAmI()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExplainAccess_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := apitypes.ExplainResponse{
		Pubkey:   "NKEY456",
		RoleName: "operator",
		IsAdmin:  false,
		Actions: []apitypes.ActionExplain{
			{Action: rbac.ActionCook, Scope: "*"},
			{Action: rbac.ActionShell, Scope: "web-*"},
		},
	}
	mockHandler(t, NatsConn, "grlx.api.auth.explain", want)

	got, err := ExplainAccess()
	if err != nil {
		t.Fatalf("ExplainAccess: %v", err)
	}
	if got.RoleName != "operator" {
		t.Fatalf("expected role operator, got %q", got.RoleName)
	}
	if got.IsAdmin {
		t.Fatal("expected IsAdmin=false")
	}
	if len(got.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(got.Actions))
	}
}

func TestExplainAccess_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.auth.explain", "not authorized")

	_, err := ExplainAccess()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListUsers_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := apitypes.UsersListResponse{
		Users: map[string]string{
			"NKEY_ALICE": "admin",
			"NKEY_BOB":   "operator",
		},
		Roles: []apitypes.RoleInfo{
			{Name: "admin", Rules: []rbac.Rule{{Action: rbac.ActionAdmin}}},
			{Name: "operator", Rules: []rbac.Rule{{Action: rbac.ActionCook, Scope: "*"}}},
		},
	}
	mockHandler(t, NatsConn, "grlx.api.auth.users", want)

	got, err := ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(got.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(got.Users))
	}
	if got.Users["NKEY_ALICE"] != "admin" {
		t.Fatalf("expected NKEY_ALICE=admin, got %q", got.Users["NKEY_ALICE"])
	}
	if len(got.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(got.Roles))
	}
}

func TestListUsers_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.auth.users", "forbidden")

	_, err := ListUsers()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAddUser_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := apitypes.UserMutateResponse{
		Success: true,
		Message: "user added",
	}
	mockHandler(t, NatsConn, "grlx.api.auth.users.add", want)

	got, err := AddUser("NKEY_NEW", "operator")
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if !got.Success {
		t.Fatal("expected success=true")
	}
	if got.Message != "user added" {
		t.Fatalf("expected message 'user added', got %q", got.Message)
	}
}

func TestAddUser_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.auth.users.add", "role not found")

	_, err := AddUser("NKEY_NEW", "nonexistent-role")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAddUser_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.users.add")

	_, err := AddUser("NKEY_NEW", "admin")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRemoveUser_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := apitypes.UserMutateResponse{
		Success: true,
		Message: "user removed",
	}
	mockHandler(t, NatsConn, "grlx.api.auth.users.remove", want)

	got, err := RemoveUser("NKEY_OLD")
	if err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	if !got.Success {
		t.Fatal("expected success=true")
	}
}

func TestRemoveUser_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.auth.users.remove", "user not found")

	_, err := RemoveUser("NKEY_MISSING")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveUser_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.users.remove")

	_, err := RemoveUser("NKEY_OLD")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- Unmarshal error paths for WhoAmI, ExplainAccess, ListUsers ---

func TestWhoAmI_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.whoami")

	_, err := WhoAmI()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestExplainAccess_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.explain")

	_, err := ExplainAccess()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListUsers_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.users")

	_, err := ListUsers()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}
