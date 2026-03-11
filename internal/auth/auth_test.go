package auth

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/rbac"
)

func TestExtractStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{"nil", nil, nil},
		{"single string", "AKEY123", []string{"AKEY123"}},
		{"string slice", []string{"A", "B"}, []string{"A", "B"}},
		{"any slice", []any{"A", "B"}, []string{"A", "B"}},
		{"mixed any slice", []any{"A", 42}, []string{"A"}},
		{"int", 42, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStringSlice(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("extractStringSlice(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractStringSlice(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestContainsKey(t *testing.T) {
	slice := []string{"AKEY1", "AKEY2", "AKEY3"}
	if !containsKey(slice, "AKEY2") {
		t.Error("expected containsKey to find AKEY2")
	}
	if containsKey(slice, "AKEY4") {
		t.Error("expected containsKey not to find AKEY4")
	}
	if containsKey(nil, "AKEY1") {
		t.Error("expected containsKey(nil, ...) to return false")
	}
}

func TestDangerouslyAllowRoot(t *testing.T) {
	if DangerouslyAllowRoot() {
		t.Error("DangerouslyAllowRoot should default to false")
	}
}

func TestWhoAmIInvalidToken(t *testing.T) {
	_, roleName, err := WhoAmI("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
	if roleName != "" {
		t.Errorf("expected empty role name, got %q", roleName)
	}
}

func TestTokenHasRouteAccessInvalidToken(t *testing.T) {
	if TokenHasRouteAccess("bad-token", "Cook") {
		t.Error("expected TokenHasRouteAccess to return false for invalid token")
	}
}

func TestTokenHasScopedAccessInvalidToken(t *testing.T) {
	if TokenHasScopedAccess("bad-token", rbac.ActionCook, []string{"web-1"}, nil) {
		t.Error("expected TokenHasScopedAccess to return false for invalid token")
	}
}

func TestTokenScopeFilterInvalidToken(t *testing.T) {
	result := TokenScopeFilter("bad-token", rbac.ActionCook, []string{"web-1"}, nil)
	if result != nil {
		t.Errorf("expected nil for invalid token, got %v", result)
	}
}

func TestLookupRoleNoPolicy(t *testing.T) {
	// With no policy loaded, lookupRole should return nil
	role := lookupRole("ANONEXISTENTKEY")
	if role != nil {
		t.Error("expected nil role for unknown key with no policy")
	}
}

func TestSetPolicyAndLookup(t *testing.T) {
	rs := rbac.NewRoleStore()
	adminRole := &rbac.Role{
		Name:  "test-admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	}
	rs.Register(adminRole)

	urm := rbac.NewUserRoleMap()
	urm.Set("ATESTPUBKEY123", "test-admin")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil) // cleanup

	role := lookupRole("ATESTPUBKEY123")
	if role == nil {
		t.Fatal("expected role for configured pubkey")
	}
	if role.Name != "test-admin" {
		t.Errorf("expected role name 'test-admin', got %q", role.Name)
	}
	if !role.HasRouteAccess("Cook") {
		t.Error("admin role should have access to Cook")
	}
	if !role.HasRouteAccess("AcceptID") {
		t.Error("admin role should have access to AcceptID")
	}

	// Unknown key still returns nil
	if lookupRole("AUNKNOWNKEY") != nil {
		t.Error("expected nil for unknown pubkey")
	}
}

func TestListAllUsersEmpty(t *testing.T) {
	SetPolicy(rbac.NewRoleStore(), rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	users := ListAllUsers()
	if users == nil {
		t.Fatal("ListAllUsers should return non-nil map")
	}
}

func TestListRolesEmpty(t *testing.T) {
	SetPolicy(rbac.NewRoleStore(), rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	roles := ListRoles()
	if len(roles) != 0 {
		t.Errorf("expected 0 roles, got %d", len(roles))
	}
}

func TestGetRoleNotFound(t *testing.T) {
	SetPolicy(rbac.NewRoleStore(), rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	_, err := GetRole("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent role")
	}
}

func TestBuiltinViewerRoleIntegration(t *testing.T) {
	// Set up policy with the built-in viewer role and a user assigned to it.
	rs := rbac.NewRoleStore()
	viewer := rbac.BuiltinViewerRole()
	if err := rs.Register(viewer); err != nil {
		t.Fatalf("failed to register viewer: %v", err)
	}

	urm := rbac.NewUserRoleMap()
	urm.Set("AVIEWERKEY123", "viewer")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	role := lookupRole("AVIEWERKEY123")
	if role == nil {
		t.Fatal("expected viewer role for configured pubkey")
	}
	if role.Name != "viewer" {
		t.Errorf("expected role name 'viewer', got %q", role.Name)
	}

	// Viewer can read
	if !role.HasRouteAccess("ListSprouts") {
		t.Error("viewer should access ListSprouts")
	}
	if !role.HasRouteAccess("GetJob") {
		t.Error("viewer should access GetJob")
	}
	if !role.HasRouteAccess("WhoAmI") {
		t.Error("viewer should access WhoAmI")
	}

	// Viewer cannot write
	if role.HasRouteAccess("Cook") {
		t.Error("viewer should not access Cook")
	}
	if role.HasRouteAccess("CmdRun") {
		t.Error("viewer should not access CmdRun")
	}
	if role.HasRouteAccess("AcceptID") {
		t.Error("viewer should not access AcceptID")
	}
	if role.HasRouteAccess("ListUsers") {
		t.Error("viewer should not access ListUsers")
	}
}

func TestViewerRoleVisibleInListRoles(t *testing.T) {
	rs := rbac.NewRoleStore()
	if err := rs.Register(rbac.BuiltinViewerRole()); err != nil {
		t.Fatalf("failed to register viewer: %v", err)
	}

	SetPolicy(rs, rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	roles := ListRoles()
	found := false
	for _, name := range roles {
		if name == "viewer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'viewer' in ListRoles output")
	}
}

func TestGetBuiltinViewerRole(t *testing.T) {
	rs := rbac.NewRoleStore()
	if err := rs.Register(rbac.BuiltinViewerRole()); err != nil {
		t.Fatalf("failed to register viewer: %v", err)
	}

	SetPolicy(rs, rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	role, err := GetRole("viewer")
	if err != nil {
		t.Fatalf("GetRole('viewer') failed: %v", err)
	}
	if role.Name != "viewer" {
		t.Errorf("expected name 'viewer', got %q", role.Name)
	}
	if len(role.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(role.Rules))
	}
}
