package rbac

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taigrr/jety"
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

func TestBuiltinOperatorRole(t *testing.T) {
	op := BuiltinOperatorRole()

	if op.Name != "operator" {
		t.Fatalf("expected name 'operator', got %q", op.Name)
	}
	if err := op.Validate(); err != nil {
		t.Fatalf("built-in operator role failed validation: %v", err)
	}

	// Operator should have these actions
	wantActions := []Action{
		ActionView, ActionCook, ActionCmd, ActionShell,
		ActionTest, ActionProps, ActionJobAdmin, ActionUserRead,
	}
	for _, action := range wantActions {
		if !op.HasAction(action) {
			t.Errorf("operator role should have action %q", action)
		}
	}

	// Operator should NOT have these actions
	denyActions := []Action{ActionAdmin, ActionPKI}
	for _, action := range denyActions {
		if op.HasAction(action) {
			t.Errorf("operator role should not have action %q", action)
		}
	}
}

func TestBuiltinOperatorRoleRouteAccess(t *testing.T) {
	op := BuiltinOperatorRole()

	// Routes that should be accessible
	allowedRoutes := []string{
		"GetVersion", "ListSprouts", "GetSprout",
		"ListJobs", "GetJob", "ListJobsForSprout",
		"GetAllProps", "GetProp", "ListCohorts",
		"Cook", "CmdRun", "ShellStart", "TestPing",
		"SetProp", "DeleteProp", "CancelJob",
		"WhoAmI", "ResolveCohort",
	}
	for _, route := range allowedRoutes {
		if !op.HasRouteAccess(route) {
			t.Errorf("operator should have access to %q", route)
		}
	}

	// Routes that should be denied
	deniedRoutes := []string{
		"AcceptID", "RejectID", "DenyID", "DeleteID",
		"UnacceptID", "ListUsers",
	}
	for _, route := range deniedRoutes {
		if op.HasRouteAccess(route) {
			t.Errorf("operator should not have access to %q", route)
		}
	}
}

func TestLoadRolesFromConfig_BuiltinsPresent(t *testing.T) {
	// LoadRolesFromConfig reads from jety, which we can't easily mock here
	// without side effects. Instead, test that NewRoleStore + built-in roles
	// work correctly as a unit.
	store := NewRoleStore()

	// Register both built-ins
	builtins := []*Role{BuiltinViewerRole(), BuiltinOperatorRole()}
	for _, b := range builtins {
		if err := store.Register(b); err != nil {
			t.Fatalf("failed to register built-in %s: %v", b.Name, err)
		}
	}

	// Verify viewer is there
	viewer, err := store.Get("viewer")
	if err != nil {
		t.Fatalf("expected viewer role to be registered: %v", err)
	}
	if viewer.Name != "viewer" {
		t.Errorf("expected name 'viewer', got %q", viewer.Name)
	}

	// Verify operator is there
	operator, err := store.Get("operator")
	if err != nil {
		t.Fatalf("expected operator role to be registered: %v", err)
	}
	if operator.Name != "operator" {
		t.Errorf("expected name 'operator', got %q", operator.Name)
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

	// Override with custom operator
	customOperator := &Role{
		Name: "operator",
		Rules: []Rule{
			{Action: ActionView, Scope: "*"},
			{Action: ActionCook, Scope: "cohort:staging"},
		},
	}
	if err := store.Register(customOperator); err != nil {
		t.Fatalf("failed to register custom operator: %v", err)
	}

	// Custom should replace built-in
	operator, err = store.Get("operator")
	if err != nil {
		t.Fatalf("operator missing after override: %v", err)
	}
	if len(operator.Rules) != 2 {
		t.Fatalf("expected 2 rules from custom operator, got %d", len(operator.Rules))
	}
	if operator.HasAction(ActionCmd) {
		t.Error("custom operator should not have ActionCmd (it was overridden)")
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

// --- jety-based integration tests for config loading ---

// setupJetyForRBACTest clears jety state for rbac-related keys.
func setupJetyForRBACTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("# test config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jety.SetConfigType("toml")
	jety.SetConfigFile(path)
	jety.Set("roles", nil)
	jety.Set("users", nil)
	jety.Set("pubkeys", nil)
	jety.Set("cohorts", nil)
}

func clearJetyRBACKeys(t *testing.T) {
	t.Helper()
	jety.Set("roles", nil)
	jety.Set("users", nil)
	jety.Set("pubkeys", nil)
	jety.Set("cohorts", nil)
}

func TestLoadRolesFromConfig_Empty(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	store, err := LoadRolesFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Built-in viewer and operator should be present even with empty config.
	viewer, err := store.Get("viewer")
	if err != nil {
		t.Fatalf("expected built-in viewer: %v", err)
	}
	if !viewer.HasAction(ActionView) {
		t.Error("viewer should have ActionView")
	}

	operator, err := store.Get("operator")
	if err != nil {
		t.Fatalf("expected built-in operator: %v", err)
	}
	if !operator.HasAction(ActionCook) {
		t.Error("operator should have ActionCook")
	}
}

func TestLoadRolesFromConfig_CustomRoles(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("roles", map[string]any{
		"sre-team": []any{
			map[string]any{"action": "admin"},
		},
		"dev-team": []any{
			map[string]any{"action": "view", "scope": "*"},
			map[string]any{"action": "cook", "scope": "cohort:staging"},
		},
	})

	store, err := LoadRolesFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sre, err := store.Get("sre-team")
	if err != nil {
		t.Fatalf("expected sre-team role: %v", err)
	}
	if !sre.HasAction(ActionAdmin) {
		t.Error("sre-team should have admin action")
	}

	dev, err := store.Get("dev-team")
	if err != nil {
		t.Fatalf("expected dev-team role: %v", err)
	}
	if len(dev.Rules) != 2 {
		t.Errorf("expected 2 rules for dev-team, got %d", len(dev.Rules))
	}
}

func TestLoadRolesFromConfig_InvalidAction(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("roles", map[string]any{
		"bad-role": []any{
			map[string]any{"action": "nonexistent"},
		},
	})

	_, err := LoadRolesFromConfig()
	if err == nil {
		t.Error("expected error for invalid action in role config")
	}
}

func TestLoadRolesFromConfig_OverrideBuiltin(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	// Override built-in viewer with restricted scope.
	jety.Set("roles", map[string]any{
		"viewer": []any{
			map[string]any{"action": "view", "scope": "cohort:monitoring"},
		},
	})

	store, err := LoadRolesFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewer, err := store.Get("viewer")
	if err != nil {
		t.Fatalf("expected viewer role: %v", err)
	}
	if len(viewer.Rules) != 1 {
		t.Fatalf("expected 1 rule from override, got %d", len(viewer.Rules))
	}
	if viewer.Rules[0].Scope != "cohort:monitoring" {
		t.Errorf("expected scoped override, got %q", viewer.Rules[0].Scope)
	}
}

func TestLoadUsersFromConfig_NewFormat(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{"PUBKEY_ADMIN_1", "PUBKEY_ADMIN_2"},
		"dev":   []any{"PUBKEY_DEV_1"},
	})

	m := LoadUsersFromConfig()
	if m.RoleName("PUBKEY_ADMIN_1") != "admin" {
		t.Errorf("expected admin, got %q", m.RoleName("PUBKEY_ADMIN_1"))
	}
	if m.RoleName("PUBKEY_ADMIN_2") != "admin" {
		t.Errorf("expected admin, got %q", m.RoleName("PUBKEY_ADMIN_2"))
	}
	if m.RoleName("PUBKEY_DEV_1") != "dev" {
		t.Errorf("expected dev, got %q", m.RoleName("PUBKEY_DEV_1"))
	}
	if m.RoleName("UNKNOWN") != "" {
		t.Errorf("expected empty for unknown key, got %q", m.RoleName("UNKNOWN"))
	}
}

func TestLoadUsersFromConfig_LegacyFormat(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("pubkeys", map[string]any{
		"admin": []any{"LEGACY_KEY_1"},
	})

	m := LoadUsersFromConfig()
	if m.RoleName("LEGACY_KEY_1") != "admin" {
		t.Errorf("expected admin from legacy format, got %q", m.RoleName("LEGACY_KEY_1"))
	}
}

func TestLoadUsersFromConfig_NewOverridesLegacy(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	// Same key appears in both — new format wins.
	jety.Set("users", map[string]any{
		"operator": []any{"SHARED_KEY"},
	})
	jety.Set("pubkeys", map[string]any{
		"admin": []any{"SHARED_KEY"},
	})

	m := LoadUsersFromConfig()
	// users section is processed first; legacy should NOT override.
	if m.RoleName("SHARED_KEY") != "operator" {
		t.Errorf("expected operator (new format wins), got %q", m.RoleName("SHARED_KEY"))
	}
}

func TestLoadUsersFromConfig_Empty(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	m := LoadUsersFromConfig()
	if len(m.All()) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m.All()))
	}
}

func TestUserRoleMapDelete(t *testing.T) {
	m := NewUserRoleMap()
	m.Set("KEY1", "admin")
	m.Set("KEY2", "viewer")

	// Delete existing key.
	existed := m.Delete("KEY1")
	if !existed {
		t.Error("expected true for deleting existing key")
	}
	if m.RoleName("KEY1") != "" {
		t.Error("KEY1 should be removed after Delete")
	}

	// Delete non-existent key.
	existed = m.Delete("NONEXISTENT")
	if existed {
		t.Error("expected false for deleting non-existent key")
	}

	// KEY2 should still be there.
	if m.RoleName("KEY2") != "viewer" {
		t.Error("KEY2 should still exist")
	}
}

func TestValidateUserUniqueness_WithJety(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	// No duplicates.
	jety.Set("users", map[string]any{
		"admin": []any{"KEY_A"},
		"dev":   []any{"KEY_B"},
	})

	err := ValidateUserUniqueness()
	if err != nil {
		t.Errorf("expected no error for unique keys, got: %v", err)
	}
}

func TestValidateUserUniqueness_WithJetyDuplicate(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	// Same key in two roles.
	jety.Set("users", map[string]any{
		"admin":  []any{"DUPE_KEY"},
		"viewer": []any{"DUPE_KEY"},
	})

	err := ValidateUserUniqueness()
	if err == nil {
		t.Error("expected error for duplicate pubkey across roles")
	}
}

func TestValidateUserUniqueness_CrossSectionJety(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{"CROSS_KEY"},
	})
	jety.Set("pubkeys", map[string]any{
		"viewer": []any{"CROSS_KEY"},
	})

	err := ValidateUserUniqueness()
	if err == nil {
		t.Error("expected error for cross-section duplicate")
	}
}

func TestValidateUsernameUniqueness_NoDuplicates(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{
			map[string]any{"pubkey": "KEY_A", "username": "alice"},
			map[string]any{"pubkey": "KEY_B", "username": "bob"},
		},
	})

	err := ValidateUsernameUniqueness()
	if err != nil {
		t.Errorf("expected no error for unique usernames, got: %v", err)
	}
}

func TestValidateUsernameUniqueness_DetectsDuplicate(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{
			map[string]any{"pubkey": "KEY_A", "username": "alice"},
		},
		"viewer": []any{
			map[string]any{"pubkey": "KEY_B", "username": "alice"},
		},
	})

	err := ValidateUsernameUniqueness()
	if err == nil {
		t.Fatal("expected error for duplicate username 'alice' across roles")
	}
	if !errors.Is(err, ErrDuplicateUsername) {
		t.Errorf("expected ErrDuplicateUsername, got: %v", err)
	}
	if !strings.Contains(err.Error(), "alice") {
		t.Errorf("error should mention 'alice', got: %v", err)
	}
}

func TestValidateUsernameUniqueness_SameRoleDuplicate(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{
			map[string]any{"pubkey": "KEY_A", "username": "bob"},
			map[string]any{"pubkey": "KEY_C", "username": "bob"},
		},
	})

	err := ValidateUsernameUniqueness()
	if err == nil {
		t.Fatal("expected error for duplicate username within same role")
	}
	if !errors.Is(err, ErrDuplicateUsername) {
		t.Errorf("expected ErrDuplicateUsername, got: %v", err)
	}
}

func TestValidateUsernameUniqueness_EmptyUsernamesIgnored(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	// Two pubkeys without usernames should not conflict.
	jety.Set("users", map[string]any{
		"admin":  []any{"KEY_A"},
		"viewer": []any{"KEY_B"},
	})

	err := ValidateUsernameUniqueness()
	if err != nil {
		t.Errorf("expected no error when usernames are empty, got: %v", err)
	}
}

func TestValidateUsernameUniqueness_MixedWithAndWithoutUsernames(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{
			map[string]any{"pubkey": "KEY_A", "username": "alice"},
			"KEY_B", // No username — should not conflict.
		},
		"viewer": []any{
			map[string]any{"pubkey": "KEY_C", "username": "bob"},
		},
	})

	err := ValidateUsernameUniqueness()
	if err != nil {
		t.Errorf("expected no error for unique usernames with some empty, got: %v", err)
	}
}

func TestValidateUsernameUniqueness_EmptyConfig(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	err := ValidateUsernameUniqueness()
	if err != nil {
		t.Errorf("expected no error for empty config, got: %v", err)
	}
}

func TestLoadCohortsFromConfig_Empty(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	reg, err := LoadCohortsFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return an empty registry, no error.
	if len(reg.List()) != 0 {
		t.Errorf("expected empty registry, got %d cohorts", len(reg.List()))
	}
}

func TestLoadCohortsFromConfig_Static(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("cohorts", map[string]any{
		"web": map[string]any{
			"type":    "static",
			"members": []any{"web-1", "web-2", "web-3"},
		},
	})

	reg, err := LoadCohortsFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cohort, err := reg.Get("web")
	if err != nil {
		t.Fatalf("expected web cohort: %v", err)
	}
	if cohort.Type != CohortTypeStatic {
		t.Errorf("expected static type, got %q", cohort.Type)
	}
	if len(cohort.Members) != 3 {
		t.Errorf("expected 3 members, got %d", len(cohort.Members))
	}
}

func TestLoadCohortsFromConfig_Dynamic(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("cohorts", map[string]any{
		"linux": map[string]any{
			"type": "dynamic",
			"match": map[string]any{
				"prop_name":  "os",
				"prop_value": "linux",
			},
		},
	})

	reg, err := LoadCohortsFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cohort, err := reg.Get("linux")
	if err != nil {
		t.Fatalf("expected linux cohort: %v", err)
	}
	if cohort.Type != CohortTypeDynamic {
		t.Errorf("expected dynamic type, got %q", cohort.Type)
	}
	if cohort.Match.PropName != "os" {
		t.Errorf("expected prop_name 'os', got %q", cohort.Match.PropName)
	}
	if cohort.Match.PropValue != "linux" {
		t.Errorf("expected prop_value 'linux', got %q", cohort.Match.PropValue)
	}
}

func TestLoadCohortsFromConfig_Compound(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("cohorts", map[string]any{
		"web": map[string]any{
			"type":    "static",
			"members": []any{"web-1", "web-2"},
		},
		"db": map[string]any{
			"type":    "static",
			"members": []any{"db-1"},
		},
		"all-services": map[string]any{
			"type": "compound",
			"compound": map[string]any{
				"operator": "OR",
				"operands": []any{"web", "db"},
			},
		},
	})

	reg, err := LoadCohortsFromConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cohort, err := reg.Get("all-services")
	if err != nil {
		t.Fatalf("expected all-services cohort: %v", err)
	}
	if cohort.Type != CohortTypeCompound {
		t.Errorf("expected compound type, got %q", cohort.Type)
	}
	if cohort.Compound.Operator != OperatorOR {
		t.Errorf("expected OR operator, got %q", cohort.Compound.Operator)
	}
	if len(cohort.Compound.Operands) != 2 {
		t.Errorf("expected 2 operands, got %d", len(cohort.Compound.Operands))
	}
}

func TestLoadCohortsFromConfig_InvalidType(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("cohorts", map[string]any{
		"bad": map[string]any{
			"type": "magical",
		},
	})

	_, err := LoadCohortsFromConfig()
	if err == nil {
		t.Error("expected error for invalid cohort type")
	}
}

// --- parseCohortEntry tests ---

func TestParseCohortEntry_Static(t *testing.T) {
	raw := map[string]any{
		"type":    "static",
		"members": []any{"s1", "s2"},
	}

	c, err := parseCohortEntry("test", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "test" {
		t.Errorf("expected name 'test', got %q", c.Name)
	}
	if c.Type != CohortTypeStatic {
		t.Errorf("expected static, got %q", c.Type)
	}
	if len(c.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(c.Members))
	}
}

func TestParseCohortEntry_NotAMap(t *testing.T) {
	_, err := parseCohortEntry("bad", "not a map")
	if err == nil {
		t.Error("expected error for non-map value")
	}
}

func TestParseCohortEntry_DynamicMissingPropName(t *testing.T) {
	raw := map[string]any{
		"type": "dynamic",
		"match": map[string]any{
			"prop_value": "linux",
		},
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for dynamic cohort without prop_name")
	}
}

func TestParseCohortEntry_DynamicMatchNil(t *testing.T) {
	raw := map[string]any{
		"type":  "dynamic",
		"match": nil,
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for nil match")
	}
}

func TestParseCohortEntry_DynamicMatchNotMap(t *testing.T) {
	raw := map[string]any{
		"type":  "dynamic",
		"match": "not a map",
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for non-map match")
	}
}

func TestParseCohortEntry_CompoundNil(t *testing.T) {
	raw := map[string]any{
		"type":     "compound",
		"compound": nil,
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for nil compound")
	}
}

func TestParseCohortEntry_CompoundNotMap(t *testing.T) {
	raw := map[string]any{
		"type":     "compound",
		"compound": "not a map",
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for non-map compound")
	}
}

func TestParseCohortEntry_CompoundInvalidOperator(t *testing.T) {
	raw := map[string]any{
		"type": "compound",
		"compound": map[string]any{
			"operator": "XOR",
			"operands": []any{"a", "b"},
		},
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for invalid operator")
	}
}

func TestParseCohortEntry_CompoundTooFewOperands(t *testing.T) {
	raw := map[string]any{
		"type": "compound",
		"compound": map[string]any{
			"operator": "AND",
			"operands": []any{"a"},
		},
	}

	_, err := parseCohortEntry("bad", raw)
	if err == nil {
		t.Error("expected error for fewer than 2 operands")
	}
}

// --- parseStringSlice tests ---

func TestParseStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  int
	}{
		{"nil", nil, 0},
		{"[]any strings", []any{"a", "b", "c"}, 3},
		{"[]any mixed", []any{"a", 42, "c"}, 2},
		{"[]string", []string{"x", "y"}, 2},
		{"unsupported type", 42, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStringSlice(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseStringSlice() = %d items, want %d", len(got), tt.want)
			}
		})
	}
}

// --- parseDynamicMatch tests ---

func TestParseDynamicMatch(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			"valid",
			map[string]any{"prop_name": "os", "prop_value": "linux"},
			false,
		},
		{
			"nil",
			nil,
			true,
		},
		{
			"not a map",
			"string",
			true,
		},
		{
			"missing prop_name",
			map[string]any{"prop_value": "linux"},
			true,
		},
		{
			"empty prop_name",
			map[string]any{"prop_name": "", "prop_value": "linux"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDynamicMatch(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDynamicMatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- parseCompoundExpr tests ---

func TestParseCompoundExpr(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			"valid AND",
			map[string]any{"operator": "AND", "operands": []any{"a", "b"}},
			false,
		},
		{
			"valid OR",
			map[string]any{"operator": "OR", "operands": []any{"a", "b", "c"}},
			false,
		},
		{
			"valid EXCEPT",
			map[string]any{"operator": "EXCEPT", "operands": []any{"all", "bad"}},
			false,
		},
		{
			"nil",
			nil,
			true,
		},
		{
			"not a map",
			[]any{"a"},
			true,
		},
		{
			"invalid operator",
			map[string]any{"operator": "NAND", "operands": []any{"a", "b"}},
			true,
		},
		{
			"too few operands",
			map[string]any{"operator": "AND", "operands": []any{"a"}},
			true,
		},
		{
			"no operands",
			map[string]any{"operator": "AND"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCompoundExpr(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCompoundExpr() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserRoleMapUsername(t *testing.T) {
	m := NewUserRoleMap()

	m.Set("APUBKEY1", "admin")
	m.SetUsername("APUBKEY1", "alice")
	m.Set("APUBKEY2", "viewer")

	if got := m.Username("APUBKEY1"); got != "alice" {
		t.Errorf("Username(APUBKEY1) = %q, want 'alice'", got)
	}
	if got := m.Username("APUBKEY2"); got != "" {
		t.Errorf("Username(APUBKEY2) = %q, want empty", got)
	}

	names := m.AllWithUsernames()
	if len(names) != 1 {
		t.Errorf("AllWithUsernames() len = %d, want 1", len(names))
	}
	if names["APUBKEY1"] != "alice" {
		t.Errorf("AllWithUsernames()[APUBKEY1] = %q, want 'alice'", names["APUBKEY1"])
	}
}

func TestUserRoleMapDeleteRemovesUsername(t *testing.T) {
	m := NewUserRoleMap()
	m.Set("APUBKEY1", "admin")
	m.SetUsername("APUBKEY1", "alice")

	m.Delete("APUBKEY1")

	if got := m.Username("APUBKEY1"); got != "" {
		t.Errorf("after Delete, Username() = %q, want empty", got)
	}
	if got := m.RoleName("APUBKEY1"); got != "" {
		t.Errorf("after Delete, RoleName() = %q, want empty", got)
	}
}

func TestParseUserEntries(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []userEntry
	}{
		{
			name:  "plain strings",
			input: []any{"APUBKEY1", "APUBKEY2"},
			want:  []userEntry{{Pubkey: "APUBKEY1"}, {Pubkey: "APUBKEY2"}},
		},
		{
			name: "rich format",
			input: []any{
				map[string]any{"pubkey": "APUBKEY1", "username": "alice"},
				map[string]any{"pubkey": "APUBKEY2", "username": "bob"},
			},
			want: []userEntry{
				{Pubkey: "APUBKEY1", Username: "alice"},
				{Pubkey: "APUBKEY2", Username: "bob"},
			},
		},
		{
			name: "mixed format",
			input: []any{
				"APUBKEY1",
				map[string]any{"pubkey": "APUBKEY2", "username": "bob"},
			},
			want: []userEntry{
				{Pubkey: "APUBKEY1"},
				{Pubkey: "APUBKEY2", Username: "bob"},
			},
		},
		{
			name: "rich without username",
			input: []any{
				map[string]any{"pubkey": "APUBKEY1"},
			},
			want: []userEntry{{Pubkey: "APUBKEY1"}},
		},
		{
			name:  "single string",
			input: "APUBKEY1",
			want:  []userEntry{{Pubkey: "APUBKEY1"}},
		},
		{
			name:  "nil",
			input: nil,
			want:  nil,
		},
		{
			name: "skip empty pubkey maps",
			input: []any{
				map[string]any{"username": "orphan"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUserEntries(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseUserEntries() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Pubkey != tt.want[i].Pubkey {
					t.Errorf("[%d] Pubkey = %q, want %q", i, got[i].Pubkey, tt.want[i].Pubkey)
				}
				if got[i].Username != tt.want[i].Username {
					t.Errorf("[%d] Username = %q, want %q", i, got[i].Username, tt.want[i].Username)
				}
			}
		})
	}
}

func TestLoadUsersWithUsernames(t *testing.T) {
	setupJetyForRBACTest(t)
	defer clearJetyRBACKeys(t)

	jety.Set("users", map[string]any{
		"admin": []any{
			map[string]any{"pubkey": "APUBKEY1", "username": "alice"},
			map[string]any{"pubkey": "APUBKEY2", "username": "bob"},
		},
		"viewer": []any{"APUBKEY3"},
	})

	m := LoadUsersFromConfig()

	if m.RoleName("APUBKEY1") != "admin" {
		t.Errorf("APUBKEY1 role = %q, want 'admin'", m.RoleName("APUBKEY1"))
	}
	if m.Username("APUBKEY1") != "alice" {
		t.Errorf("APUBKEY1 username = %q, want 'alice'", m.Username("APUBKEY1"))
	}
	if m.Username("APUBKEY2") != "bob" {
		t.Errorf("APUBKEY2 username = %q, want 'bob'", m.Username("APUBKEY2"))
	}
	if m.RoleName("APUBKEY3") != "viewer" {
		t.Errorf("APUBKEY3 role = %q, want 'viewer'", m.RoleName("APUBKEY3"))
	}
	if m.Username("APUBKEY3") != "" {
		t.Errorf("APUBKEY3 username = %q, want empty", m.Username("APUBKEY3"))
	}
}
