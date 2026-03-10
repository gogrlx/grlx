package rbac

import (
	"testing"
)

func TestRoleStoreRegisterAndGet(t *testing.T) {
	store := NewRoleStore()

	role := &Role{
		Name: "dev",
		Rules: []Rule{
			{Action: ActionView, Scope: "*"},
			{Action: ActionCook, Scope: "cohort:staging"},
		},
	}
	if err := store.Register(role); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := store.Get("dev")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "dev" {
		t.Errorf("expected name 'dev', got %q", got.Name)
	}
	if len(got.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(got.Rules))
	}
}

func TestRoleStoreGetNotFound(t *testing.T) {
	store := NewRoleStore()
	_, err := store.Get("missing")
	if err == nil {
		t.Error("expected error for missing role")
	}
}

func TestRoleStoreList(t *testing.T) {
	store := NewRoleStore()
	store.Register(&Role{Name: "a", Rules: []Rule{{Action: ActionView}}})
	store.Register(&Role{Name: "b", Rules: []Rule{{Action: ActionAdmin}}})

	names := store.List()
	if len(names) != 2 {
		t.Errorf("expected 2 roles, got %d", len(names))
	}
}

func TestRoleStoreRejectInvalid(t *testing.T) {
	store := NewRoleStore()
	err := store.Register(&Role{Name: "", Rules: []Rule{{Action: ActionView}}})
	if err == nil {
		t.Error("expected error for role with no name")
	}
}

func TestUserRoleMap(t *testing.T) {
	m := NewUserRoleMap()
	m.Set("APUBKEY1", "admin")
	m.Set("APUBKEY2", "viewer")

	if m.RoleName("APUBKEY1") != "admin" {
		t.Error("expected admin for APUBKEY1")
	}
	if m.RoleName("APUBKEY2") != "viewer" {
		t.Error("expected viewer for APUBKEY2")
	}
	if m.RoleName("APUBKEY3") != "" {
		t.Error("expected empty for unknown key")
	}

	all := m.All()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestParseRoleEntry(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			"valid rules",
			[]any{
				map[string]any{"action": "view", "scope": "*"},
				map[string]any{"action": "cook", "scope": "cohort:web"},
				map[string]any{"action": "cmd", "scope": "sprout:db-1"},
			},
			false,
		},
		{
			"empty scope defaults to *",
			[]any{
				map[string]any{"action": "view"},
			},
			false,
		},
		{
			"invalid action",
			[]any{
				map[string]any{"action": "superpower"},
			},
			true,
		},
		{
			"not a list",
			"invalid",
			true,
		},
		{
			"rule not a map",
			[]any{"invalid"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := parseRoleEntry("test-role", tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRoleEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && role.Name != "test-role" {
				t.Errorf("expected name 'test-role', got %q", role.Name)
			}
		})
	}
}
