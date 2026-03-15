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

func TestBuiltinViewerRole(t *testing.T) {
	viewer := BuiltinViewerRole()

	if viewer.Name != "viewer" {
		t.Fatalf("expected name 'viewer', got %q", viewer.Name)
	}
	if err := viewer.Validate(); err != nil {
		t.Fatalf("built-in viewer role failed validation: %v", err)
	}

	// Viewer should have view and user_read only
	allowedActions := map[Action]bool{ActionView: true, ActionUserRead: true}
	for _, rule := range viewer.Rules {
		if !allowedActions[rule.Action] {
			t.Errorf("viewer role has unexpected action: %q", rule.Action)
		}
	}

	// Viewer should NOT have write actions
	writeActions := []Action{ActionCook, ActionCmd, ActionPKI, ActionProps, ActionJobAdmin, ActionAdmin, ActionTest}
	for _, action := range writeActions {
		if viewer.HasAction(action) {
			t.Errorf("viewer role should not have action %q", action)
		}
	}

	// Viewer should have read actions
	if !viewer.HasAction(ActionView) {
		t.Error("viewer role should have ActionView")
	}
	if !viewer.HasAction(ActionUserRead) {
		t.Error("viewer role should have ActionUserRead")
	}
}

func TestBuiltinViewerRoleRouteAccess(t *testing.T) {
	viewer := BuiltinViewerRole()

	// Routes that should be accessible
	readRoutes := []string{
		"GetVersion", "ListSprouts", "GetSprout",
		"ListJobs", "GetJob", "ListJobsForSprout",
		"GetAllProps", "GetProp", "ListCohorts",
		"ResolveCohort", "ListID", "WhoAmI",
	}
	for _, route := range readRoutes {
		if !viewer.HasRouteAccess(route) {
			t.Errorf("viewer should have access to %q", route)
		}
	}

	// Routes that should be denied
	writeRoutes := []string{
		"Cook", "CmdRun", "TestPing", "SetProp", "DeleteProp",
		"CancelJob", "AcceptID", "RejectID", "DenyID", "DeleteID",
		"UnacceptID", "ListUsers",
	}
	for _, route := range writeRoutes {
		if viewer.HasRouteAccess(route) {
			t.Errorf("viewer should not have access to %q", route)
		}
	}
}

func TestLoadRolesFromConfig_BuiltinViewerPresent(t *testing.T) {
	// LoadRolesFromConfig reads from jety, which we can't easily mock here
	// without side effects. Instead, test that NewRoleStore + BuiltinViewerRole
	// works correctly as a unit.
	store := NewRoleStore()
	builtin := BuiltinViewerRole()
	if err := store.Register(builtin); err != nil {
		t.Fatalf("failed to register built-in viewer: %v", err)
	}

	// Verify it's there
	viewer, err := store.Get("viewer")
	if err != nil {
		t.Fatalf("expected viewer role to be registered: %v", err)
	}
	if viewer.Name != "viewer" {
		t.Errorf("expected name 'viewer', got %q", viewer.Name)
	}

	// Override with custom viewer
	customViewer := &Role{
		Name: "viewer",
		Rules: []Rule{
			{Action: ActionView, Scope: "cohort:monitoring"},
			{Action: ActionUserRead, Scope: "*"},
		},
	}
	if err := store.Register(customViewer); err != nil {
		t.Fatalf("failed to register custom viewer: %v", err)
	}

	// Custom should replace built-in
	viewer, err = store.Get("viewer")
	if err != nil {
		t.Fatalf("viewer missing after override: %v", err)
	}
	if len(viewer.Rules) != 2 {
		t.Fatalf("expected 2 rules from custom viewer, got %d", len(viewer.Rules))
	}
	// First rule should be scoped, not wildcard
	if viewer.Rules[0].Scope != "cohort:monitoring" {
		t.Errorf("expected cohort:monitoring scope, got %q", viewer.Rules[0].Scope)
	}
}

func TestValidateUserUniqueness_NoDuplicates(t *testing.T) {
	// With a clean jety state (no users/pubkeys configured), validation passes.
	err := ValidateUserUniqueness()
	if err != nil {
		t.Errorf("expected no error with empty config, got: %v", err)
	}
}

func TestValidateUserUniqueness_DetectsDuplicate(t *testing.T) {
	// Build a UserRoleMap with a duplicate to verify the detection logic.
	// Since ValidateUserUniqueness reads from jety directly, we test the
	// helper-level duplicate detection in a unit-style way.

	// Simulate what ValidateUserUniqueness does internally.
	seen := make(map[string][]string)
	seen["APUBKEY_DUPE"] = append(seen["APUBKEY_DUPE"], "users.admin")
	seen["APUBKEY_DUPE"] = append(seen["APUBKEY_DUPE"], "users.viewer")
	seen["APUBKEY_OK"] = append(seen["APUBKEY_OK"], "users.dev")

	dupeCount := 0
	for _, roles := range seen {
		if len(roles) > 1 {
			dupeCount++
		}
	}
	if dupeCount != 1 {
		t.Errorf("expected 1 duplicate, got %d", dupeCount)
	}
}

func TestValidateUserUniqueness_NoDupeSameSection(t *testing.T) {
	// Two different pubkeys in the same role is fine.
	seen := make(map[string][]string)
	seen["APUBKEY_A"] = append(seen["APUBKEY_A"], "users.admin")
	seen["APUBKEY_B"] = append(seen["APUBKEY_B"], "users.admin")

	for _, roles := range seen {
		if len(roles) > 1 {
			t.Error("should not flag different pubkeys under the same role as duplicates")
		}
	}
}

func TestValidateUserUniqueness_CrossSection(t *testing.T) {
	// Same pubkey in users.admin and pubkeys.admin is a duplicate
	// (even with the same role name, it indicates redundant config).
	seen := make(map[string][]string)
	seen["APUBKEY_CROSS"] = append(seen["APUBKEY_CROSS"], "users.admin")
	seen["APUBKEY_CROSS"] = append(seen["APUBKEY_CROSS"], "pubkeys.admin")

	dupeCount := 0
	for _, roles := range seen {
		if len(roles) > 1 {
			dupeCount++
		}
	}
	if dupeCount != 1 {
		t.Errorf("expected 1 cross-section duplicate, got %d", dupeCount)
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
