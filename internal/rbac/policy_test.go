package rbac

import (
	"testing"
)

func newTestPolicy() *Policy {
	store := NewRoleStore()
	store.Register(&Role{
		Name:  "admin",
		Rules: []Rule{{Action: ActionAdmin, Scope: "*"}},
	})
	store.Register(&Role{
		Name: "dev",
		Rules: []Rule{
			{Action: ActionView, Scope: "*"},
			{Action: ActionCook, Scope: "cohort:staging"},
			{Action: ActionUserRead, Scope: "*"},
		},
	})
	store.Register(&Role{
		Name:  "viewer",
		Rules: []Rule{{Action: ActionView, Scope: "*"}, {Action: ActionUserRead, Scope: "*"}},
	})

	users := NewUserRoleMap()
	users.Set("APUBKEY_ADMIN", "admin")
	users.Set("APUBKEY_DEV", "dev")
	users.Set("APUBKEY_VIEWER", "viewer")

	reg := NewRegistry()
	reg.Register(&Cohort{
		Name:    "staging",
		Type:    CohortTypeStatic,
		Members: []string{"staging-1", "staging-2"},
	})

	return &Policy{
		Roles:   store,
		Users:   users,
		Cohorts: reg,
	}
}

func TestValidatePolicy_Clean(t *testing.T) {
	p := newTestPolicy()
	warnings := ValidatePolicy(p)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for clean policy, got %d: %v", len(warnings), warnings)
	}
}

func TestValidatePolicy_OrphanRoleRef(t *testing.T) {
	p := newTestPolicy()
	p.Users.Set("APUBKEY_GHOST", "nonexistent-role")

	warnings := ValidatePolicy(p)
	found := false
	for _, w := range warnings {
		if w.Kind == "orphan_role_ref" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected orphan_role_ref warning for user referencing nonexistent role")
	}
}

func TestValidatePolicy_OrphanCohortRef(t *testing.T) {
	store := NewRoleStore()
	store.Register(&Role{
		Name:  "bad-scope",
		Rules: []Rule{{Action: ActionCook, Scope: "cohort:production"}},
	})

	users := NewUserRoleMap()
	users.Set("APUBKEY1", "bad-scope")

	reg := NewRegistry()
	// "production" cohort is NOT registered

	p := &Policy{Roles: store, Users: users, Cohorts: reg}
	warnings := ValidatePolicy(p)

	found := false
	for _, w := range warnings {
		if w.Kind == "orphan_cohort_ref" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected orphan_cohort_ref warning for role referencing nonexistent cohort")
	}
}

func TestValidatePolicy_EmptyRole(t *testing.T) {
	store := NewRoleStore()
	store.Register(&Role{Name: "empty", Rules: []Rule{}})

	users := NewUserRoleMap()
	users.Set("APUBKEY1", "empty")

	p := &Policy{Roles: store, Users: users, Cohorts: NewRegistry()}
	warnings := ValidatePolicy(p)

	found := false
	for _, w := range warnings {
		if w.Kind == "empty_role" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected empty_role warning")
	}
}

func TestValidatePolicy_UnusedRole(t *testing.T) {
	store := NewRoleStore()
	store.Register(&Role{
		Name:  "admin",
		Rules: []Rule{{Action: ActionAdmin, Scope: "*"}},
	})
	store.Register(&Role{
		Name:  "orphan",
		Rules: []Rule{{Action: ActionView, Scope: "*"}},
	})

	users := NewUserRoleMap()
	users.Set("APUBKEY1", "admin")
	// Nobody references "orphan"

	p := &Policy{Roles: store, Users: users, Cohorts: NewRegistry()}
	warnings := ValidatePolicy(p)

	found := false
	for _, w := range warnings {
		if w.Kind == "unused_role" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected unused_role warning for unreferenced role")
	}
}

func TestValidatePolicy_NilStores(t *testing.T) {
	p := &Policy{Roles: nil, Users: nil, Cohorts: nil}
	warnings := ValidatePolicy(p)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil stores, got %d", len(warnings))
	}
}

func TestValidatePolicy_NilCohortRegistry(t *testing.T) {
	store := NewRoleStore()
	store.Register(&Role{
		Name:  "scoped",
		Rules: []Rule{{Action: ActionCook, Scope: "cohort:web"}},
	})

	users := NewUserRoleMap()
	users.Set("APUBKEY1", "scoped")

	// Cohorts is nil — scope references should be silently skipped
	p := &Policy{Roles: store, Users: users, Cohorts: nil}
	warnings := ValidatePolicy(p)

	// Should NOT produce orphan_cohort_ref when registry is nil
	for _, w := range warnings {
		if w.Kind == "orphan_cohort_ref" {
			t.Error("should not produce orphan_cohort_ref when cohort registry is nil")
		}
	}
}

func TestExplainAccess_Admin(t *testing.T) {
	p := newTestPolicy()
	summary := ExplainAccess(p, "APUBKEY_ADMIN")

	if summary.RoleName != "admin" {
		t.Errorf("expected role 'admin', got %q", summary.RoleName)
	}
	if !summary.IsAdmin {
		t.Error("expected IsAdmin = true for admin user")
	}
	if len(summary.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(summary.Actions))
	}
}

func TestExplainAccess_ScopedUser(t *testing.T) {
	p := newTestPolicy()
	summary := ExplainAccess(p, "APUBKEY_DEV")

	if summary.RoleName != "dev" {
		t.Errorf("expected role 'dev', got %q", summary.RoleName)
	}
	if summary.IsAdmin {
		t.Error("dev user should not be admin")
	}
	if len(summary.Actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(summary.Actions))
	}

	// Check that cook is scoped to cohort:staging
	foundScopedCook := false
	for _, a := range summary.Actions {
		if a.Action == ActionCook && a.Scope == "cohort:staging" {
			foundScopedCook = true
		}
	}
	if !foundScopedCook {
		t.Error("expected cook action scoped to cohort:staging")
	}
}

func TestExplainAccess_UnknownUser(t *testing.T) {
	p := newTestPolicy()
	summary := ExplainAccess(p, "APUBKEY_UNKNOWN")

	if summary.RoleName != "" {
		t.Errorf("expected empty role, got %q", summary.RoleName)
	}
	if len(summary.Warnings) == 0 {
		t.Error("expected warnings for unknown user")
	}
	if summary.Warnings[0].Kind != "no_role" {
		t.Errorf("expected no_role warning, got %q", summary.Warnings[0].Kind)
	}
}

func TestExplainAccess_OrphanRole(t *testing.T) {
	store := NewRoleStore()
	// Role "ghost" is NOT registered
	users := NewUserRoleMap()
	users.Set("APUBKEY1", "ghost")

	p := &Policy{Roles: store, Users: users, Cohorts: NewRegistry()}
	summary := ExplainAccess(p, "APUBKEY1")

	if summary.RoleName != "ghost" {
		t.Errorf("expected role 'ghost', got %q", summary.RoleName)
	}
	if len(summary.Warnings) == 0 {
		t.Error("expected warnings for orphan role reference")
	}
	if summary.Warnings[0].Kind != "orphan_role_ref" {
		t.Errorf("expected orphan_role_ref warning, got %q", summary.Warnings[0].Kind)
	}
}

func TestExplainAccess_NilPolicy(t *testing.T) {
	p := &Policy{Roles: nil, Users: nil}
	summary := ExplainAccess(p, "APUBKEY1")

	if len(summary.Warnings) == 0 {
		t.Error("expected warnings for nil policy")
	}
	if summary.Warnings[0].Kind != "no_policy" {
		t.Errorf("expected no_policy warning, got %q", summary.Warnings[0].Kind)
	}
}

func TestExplainAllUsers(t *testing.T) {
	p := newTestPolicy()
	summaries := ExplainAllUsers(p)

	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}

	// Should be sorted by pubkey
	for i := 1; i < len(summaries); i++ {
		if summaries[i].Pubkey < summaries[i-1].Pubkey {
			t.Error("summaries should be sorted by pubkey")
		}
	}
}

func TestExplainAllUsers_NilUsers(t *testing.T) {
	p := &Policy{Roles: NewRoleStore(), Users: nil}
	summaries := ExplainAllUsers(p)
	if summaries != nil {
		t.Errorf("expected nil for nil UserRoleMap, got %v", summaries)
	}
}

func TestTruncatePubkey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "short"},
		{"AAAAAAAAAAAA", "AAAAAAAAAAAA"},     // exactly 12
		{"AAAAAAAAAAAAB", "AAAAAA...AAAAAB"}, // 13
		{"APUBKEYABCDEFGHIJKLMNOP", "APUBKE...KLMNOP"},
	}
	for _, tt := range tests {
		got := truncatePubkey(tt.input)
		if got != tt.want {
			t.Errorf("truncatePubkey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUserRoleMap_AllRoleNames(t *testing.T) {
	m := NewUserRoleMap()
	m.Set("KEY1", "admin")
	m.Set("KEY2", "dev")
	m.Set("KEY3", "admin") // duplicate role

	names := m.allRoleNames()
	if len(names) != 2 {
		t.Errorf("expected 2 unique role names, got %d: %v", len(names), names)
	}
}
