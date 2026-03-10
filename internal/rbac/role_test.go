package rbac

import (
	"testing"
)

func TestIsValidAction(t *testing.T) {
	tests := []struct {
		action Action
		valid  bool
	}{
		{ActionAdmin, true},
		{ActionView, true},
		{ActionCook, true},
		{ActionCmd, true},
		{ActionTest, true},
		{ActionProps, true},
		{ActionJobAdmin, true},
		{ActionPKI, true},
		{ActionUserRead, true},
		{Action("superuser"), false},
		{Action(""), false},
	}
	for _, tt := range tests {
		if got := IsValidAction(tt.action); got != tt.valid {
			t.Errorf("IsValidAction(%q) = %v, want %v", tt.action, got, tt.valid)
		}
	}
}

func TestParseAction(t *testing.T) {
	tests := []struct {
		input string
		want  Action
		err   bool
	}{
		{"admin", ActionAdmin, false},
		{"view", ActionView, false},
		{"cook", ActionCook, false},
		{"cmd", ActionCmd, false},
		{"root", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := ParseAction(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ParseAction(%q) error = %v, wantErr %v", tt.input, err, tt.err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseAction(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRouteAction(t *testing.T) {
	tests := []struct {
		route string
		want  Action
	}{
		{"GetVersion", ActionView},
		{"Cook", ActionCook},
		{"CmdRun", ActionCmd},
		{"AcceptID", ActionPKI},
		{"WhoAmI", ActionUserRead},
		{"ListUsers", ActionAdmin},
		{"UnknownRoute", ActionAdmin},
	}
	for _, tt := range tests {
		got := RouteAction(tt.route)
		if got != tt.want {
			t.Errorf("RouteAction(%q) = %q, want %q", tt.route, got, tt.want)
		}
	}
}

func TestRoleHasRouteAccess(t *testing.T) {
	adminRole := &Role{
		Name:  "admin",
		Rules: []Rule{{Action: ActionAdmin, Scope: "*"}},
	}
	viewerRole := &Role{
		Name:  "viewer",
		Rules: []Rule{{Action: ActionView, Scope: "*"}, {Action: ActionUserRead, Scope: "*"}},
	}
	operatorRole := &Role{
		Name: "operator",
		Rules: []Rule{
			{Action: ActionView, Scope: "*"},
			{Action: ActionCook, Scope: "*"},
			{Action: ActionCmd, Scope: "*"},
			{Action: ActionTest, Scope: "*"},
			{Action: ActionProps, Scope: "*"},
			{Action: ActionJobAdmin, Scope: "*"},
			{Action: ActionUserRead, Scope: "*"},
		},
	}

	tests := []struct {
		role  *Role
		route string
		want  bool
	}{
		// Admin gets everything
		{adminRole, "GetVersion", true},
		{adminRole, "Cook", true},
		{adminRole, "AcceptID", true},
		{adminRole, "ListUsers", true},

		// Viewer: read-only
		{viewerRole, "GetVersion", true},
		{viewerRole, "ListSprouts", true},
		{viewerRole, "ListJobs", true},
		{viewerRole, "WhoAmI", true},
		{viewerRole, "Cook", false},
		{viewerRole, "CmdRun", false},
		{viewerRole, "AcceptID", false},
		{viewerRole, "ListUsers", false},

		// Operator: cook/cmd but not PKI
		{operatorRole, "GetVersion", true},
		{operatorRole, "Cook", true},
		{operatorRole, "CmdRun", true},
		{operatorRole, "TestPing", true},
		{operatorRole, "SetProp", true},
		{operatorRole, "CancelJob", true},
		{operatorRole, "AcceptID", false},
		{operatorRole, "DeleteID", false},
		{operatorRole, "ListUsers", false},
	}
	for _, tt := range tests {
		got := tt.role.HasRouteAccess(tt.route)
		if got != tt.want {
			t.Errorf("Role(%q).HasRouteAccess(%q) = %v, want %v", tt.role.Name, tt.route, got, tt.want)
		}
	}
}

func TestHasScopedAccess(t *testing.T) {
	// Role: can cook on cohort:web, cmd on sprout:db-1, view everywhere
	role := &Role{
		Name: "scoped-user",
		Rules: []Rule{
			{Action: ActionView, Scope: "*"},
			{Action: ActionCook, Scope: "cohort:web"},
			{Action: ActionCmd, Scope: "sprout:db-1"},
		},
	}

	resolver := func(name string) (map[string]bool, error) {
		if name == "web" {
			return map[string]bool{"web-1": true, "web-2": true}, nil
		}
		return nil, ErrCohortNotFound
	}

	tests := []struct {
		action   Action
		sproutID string
		want     bool
	}{
		// View is global
		{ActionView, "web-1", true},
		{ActionView, "db-1", true},
		{ActionView, "anything", true},

		// Cook is scoped to cohort:web
		{ActionCook, "web-1", true},
		{ActionCook, "web-2", true},
		{ActionCook, "db-1", false},
		{ActionCook, "random", false},

		// Cmd is scoped to sprout:db-1
		{ActionCmd, "db-1", true},
		{ActionCmd, "web-1", false},
		{ActionCmd, "random", false},

		// Props not granted at all
		{ActionProps, "web-1", false},
		{ActionProps, "db-1", false},
	}
	for _, tt := range tests {
		got := role.HasScopedAccess(tt.action, tt.sproutID, resolver)
		if got != tt.want {
			t.Errorf("HasScopedAccess(%q, %q) = %v, want %v", tt.action, tt.sproutID, got, tt.want)
		}
	}
}

func TestHasScopedAccessMulti(t *testing.T) {
	role := &Role{
		Name:  "web-cook",
		Rules: []Rule{{Action: ActionCook, Scope: "cohort:web"}},
	}

	resolver := func(name string) (map[string]bool, error) {
		if name == "web" {
			return map[string]bool{"web-1": true, "web-2": true}, nil
		}
		return nil, ErrCohortNotFound
	}

	// All in scope → true
	if !role.HasScopedAccessMulti(ActionCook, []string{"web-1", "web-2"}, resolver) {
		t.Error("expected true for all-in-scope sprouts")
	}

	// One out of scope → false
	if role.HasScopedAccessMulti(ActionCook, []string{"web-1", "db-1"}, resolver) {
		t.Error("expected false when one sprout is out of scope")
	}
}

func TestScopeFilter(t *testing.T) {
	role := &Role{
		Name:  "web-cook",
		Rules: []Rule{{Action: ActionCook, Scope: "cohort:web"}},
	}

	resolver := func(name string) (map[string]bool, error) {
		if name == "web" {
			return map[string]bool{"web-1": true, "web-2": true}, nil
		}
		return nil, ErrCohortNotFound
	}

	got := role.ScopeFilter(ActionCook, []string{"web-1", "web-2", "db-1", "random"}, resolver)
	if len(got) != 2 {
		t.Fatalf("expected 2 allowed sprouts, got %d: %v", len(got), got)
	}
}

func TestAdminRoleBypassesScope(t *testing.T) {
	admin := &Role{
		Name:  "admin",
		Rules: []Rule{{Action: ActionAdmin, Scope: "*"}},
	}

	// Admin should have access to everything regardless of scope
	if !admin.HasScopedAccess(ActionCook, "any-sprout", nil) {
		t.Error("admin should have scoped access to any sprout for any action")
	}
	if !admin.HasScopedAccess(ActionPKI, "any-sprout", nil) {
		t.Error("admin should have PKI access")
	}
}

func TestValidateRole(t *testing.T) {
	tests := []struct {
		name string
		role Role
		err  bool
	}{
		{"valid", Role{Name: "test", Rules: []Rule{{Action: ActionView, Scope: "*"}}}, false},
		{"no name", Role{Rules: []Rule{{Action: ActionView}}}, true},
		{"bad action", Role{Name: "test", Rules: []Rule{{Action: "badaction"}}}, true},
		{"bad scope", Role{Name: "test", Rules: []Rule{{Action: ActionView, Scope: "invalid:format"}}}, true},
		{"empty scope ok", Role{Name: "test", Rules: []Rule{{Action: ActionView, Scope: ""}}}, false},
		{"sprout scope", Role{Name: "test", Rules: []Rule{{Action: ActionCmd, Scope: "sprout:db-1"}}}, false},
		{"cohort scope", Role{Name: "test", Rules: []Rule{{Action: ActionCook, Scope: "cohort:web"}}}, false},
		{"bare sprout:", Role{Name: "test", Rules: []Rule{{Action: ActionView, Scope: "sprout:"}}}, true},
		{"bare cohort:", Role{Name: "test", Rules: []Rule{{Action: ActionView, Scope: "cohort:"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if (err != nil) != tt.err {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.err)
			}
		})
	}
}

func TestNilResolver(t *testing.T) {
	role := &Role{
		Name:  "cohort-user",
		Rules: []Rule{{Action: ActionCook, Scope: "cohort:web"}},
	}

	// With nil resolver, cohort scopes can't be checked
	if role.HasScopedAccess(ActionCook, "web-1", nil) {
		t.Error("expected false when resolver is nil for cohort scope")
	}
}
