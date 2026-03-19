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
