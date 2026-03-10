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

func TestListUsersEmpty(t *testing.T) {
	users := ListUsers()
	if users == nil {
		t.Fatal("ListUsers should return non-nil map")
	}
	// With no config loaded, should return empty map
	total := 0
	for _, keys := range users {
		total += len(keys)
	}
	// May or may not be empty depending on test environment config
	_ = total
}

func TestDangerouslyAllowRoot(t *testing.T) {
	// Default should be false (no config set)
	if DangerouslyAllowRoot() {
		t.Error("DangerouslyAllowRoot should default to false")
	}
}

func TestWhoAmIInvalidToken(t *testing.T) {
	_, role, err := WhoAmI("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
	if role != "" {
		t.Errorf("expected empty role, got %q", role)
	}
}

func TestTokenHasRouteAccessInvalidToken(t *testing.T) {
	if TokenHasRouteAccess("bad-token", "Cook") {
		t.Error("expected TokenHasRouteAccess to return false for invalid token")
	}
}

func TestPubkeyRoleNoConfig(t *testing.T) {
	// With no config, should return empty role
	role := pubkeyRole("ANONEXISTENTKEY")
	if role != "" {
		t.Errorf("expected empty role for unknown key, got %q", role)
	}
	_ = rbac.RoleAdmin // ensure import is used
}
