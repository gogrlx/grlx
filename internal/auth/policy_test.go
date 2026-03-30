package auth

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// makeValidToken creates a signed, valid token from a fresh keypair.
// Returns the token string and the public key.
func makeValidToken(t *testing.T) (string, string) {
	t.Helper()
	kp := mustCreateKeyPair(t)
	pk, err := kp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	token, err := createSignedToken(kp)
	if err != nil {
		t.Fatal(err)
	}
	return token, pk
}

func TestTokenHasAccessWithValidAdminToken(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	admin := &rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	}
	rs.Register(admin)

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "admin")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	if !TokenHasAccess(token, "GET") {
		t.Error("TokenHasAccess should return true for valid admin token")
	}
}

func TestTokenHasAccessWithUnknownPubkey(t *testing.T) {
	token, _ := makeValidToken(t)

	// Set up a policy but don't add the token's pubkey.
	rs := rbac.NewRoleStore()
	rs.Register(&rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	})
	SetPolicy(rs, rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	if TokenHasAccess(token, "GET") {
		t.Error("TokenHasAccess should return false for unknown pubkey")
	}
}

func TestTokenHasRouteAccessWithValidToken(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	admin := &rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	}
	rs.Register(admin)

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "admin")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	if !TokenHasRouteAccess(token, "Cook") {
		t.Error("admin token should have access to Cook route")
	}
	if !TokenHasRouteAccess(token, "AcceptID") {
		t.Error("admin token should have access to AcceptID route")
	}
}

func TestTokenHasRouteAccessViewerRestrictions(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	rs.Register(rbac.BuiltinViewerRole())

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "viewer")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	// Viewer can read.
	if !TokenHasRouteAccess(token, "ListSprouts") {
		t.Error("viewer should access ListSprouts")
	}

	// Viewer cannot cook.
	if TokenHasRouteAccess(token, "Cook") {
		t.Error("viewer should not access Cook")
	}
}

func TestTokenHasActionWithValidToken(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	operator := &rbac.Role{
		Name: "operator",
		Rules: []rbac.Rule{
			{Action: rbac.ActionCook, Scope: "*"},
			{Action: rbac.ActionView, Scope: "*"},
		},
	}
	rs.Register(operator)

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "operator")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	if !TokenHasAction(token, rbac.ActionCook) {
		t.Error("operator should have cook action")
	}
	if !TokenHasAction(token, rbac.ActionView) {
		t.Error("operator should have view action")
	}
	if TokenHasAction(token, rbac.ActionAdmin) {
		t.Error("operator should not have admin action")
	}
}

func TestTokenHasScopedAccessWithCohort(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	scoped := &rbac.Role{
		Name: "web-operator",
		Rules: []rbac.Rule{
			{Action: rbac.ActionCook, Scope: "cohort:web-servers"},
		},
	}
	rs.Register(scoped)

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "web-operator")

	// Create cohort registry with a static cohort.
	reg := rbac.NewRegistry()
	reg.Register(&rbac.Cohort{
		Name:    "web-servers",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"web-1", "web-2", "web-3"},
	})

	SetPolicy(rs, urm, reg)
	defer SetPolicy(nil, nil, nil)

	allSprouts := []string{"web-1", "web-2", "web-3", "db-1", "db-2"}

	// Should have access to web sprouts.
	if !TokenHasScopedAccess(token, rbac.ActionCook, []string{"web-1"}, allSprouts) {
		t.Error("web-operator should have access to web-1")
	}

	// Should NOT have access to db sprouts.
	if TokenHasScopedAccess(token, rbac.ActionCook, []string{"db-1"}, allSprouts) {
		t.Error("web-operator should not have access to db-1")
	}
}

func TestTokenScopeFilterWithCohort(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	scoped := &rbac.Role{
		Name: "web-operator",
		Rules: []rbac.Rule{
			{Action: rbac.ActionCook, Scope: "cohort:web-servers"},
		},
	}
	rs.Register(scoped)

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "web-operator")

	reg := rbac.NewRegistry()
	reg.Register(&rbac.Cohort{
		Name:    "web-servers",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"web-1", "web-2"},
	})

	SetPolicy(rs, urm, reg)
	defer SetPolicy(nil, nil, nil)

	allSprouts := []string{"web-1", "web-2", "db-1"}
	filtered := TokenScopeFilter(token, rbac.ActionCook, []string{"web-1", "web-2", "db-1"}, allSprouts)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered sprouts, got %d: %v", len(filtered), filtered)
	}
	for _, id := range filtered {
		if id != "web-1" && id != "web-2" {
			t.Errorf("unexpected sprout in filter: %q", id)
		}
	}
}

func TestTokenScopeFilterWildcard(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	admin := &rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	}
	rs.Register(admin)

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "admin")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	sprouts := []string{"a", "b", "c"}
	filtered := TokenScopeFilter(token, rbac.ActionCook, sprouts, sprouts)
	if len(filtered) != 3 {
		t.Errorf("admin should see all sprouts, got %d", len(filtered))
	}
}

func TestWhoAmIWithValidToken(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	rs.Register(&rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	})

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "admin")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	gotPK, roleName, _, err := WhoAmI(token)
	if err != nil {
		t.Fatalf("WhoAmI() error: %v", err)
	}
	if gotPK != pk {
		t.Errorf("WhoAmI() pubkey = %q, want %q", gotPK, pk)
	}
	if roleName != "admin" {
		t.Errorf("WhoAmI() role = %q, want 'admin'", roleName)
	}
}

func TestWhoAmIUnknownUser(t *testing.T) {
	token, _ := makeValidToken(t)

	SetPolicy(rbac.NewRoleStore(), rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	_, roleName, _, err := WhoAmI(token)
	if err != nil {
		t.Fatalf("WhoAmI() error: %v", err)
	}
	if roleName != "" {
		t.Errorf("WhoAmI() role = %q, want empty for unknown user", roleName)
	}
}

func TestWhoAmIWithUsername(t *testing.T) {
	token, pk := makeValidToken(t)

	rs := rbac.NewRoleStore()
	rs.Register(&rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	})

	urm := rbac.NewUserRoleMap()
	urm.Set(pk, "admin")
	urm.SetUsername(pk, "alice")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	_, roleName, username, err := WhoAmI(token)
	if err != nil {
		t.Fatalf("WhoAmI() error: %v", err)
	}
	if roleName != "admin" {
		t.Errorf("WhoAmI() role = %q, want 'admin'", roleName)
	}
	if username != "alice" {
		t.Errorf("WhoAmI() username = %q, want 'alice'", username)
	}
}

func TestCurrentPolicy(t *testing.T) {
	rs := rbac.NewRoleStore()
	urm := rbac.NewUserRoleMap()

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	p := CurrentPolicy()
	if p == nil {
		t.Fatal("CurrentPolicy() returned nil")
	}
	if p.Roles != rs {
		t.Error("CurrentPolicy().Roles mismatch")
	}
	if p.Users != urm {
		t.Error("CurrentPolicy().Users mismatch")
	}
}

func TestCohortResolverNilRegistry(t *testing.T) {
	SetPolicy(rbac.NewRoleStore(), rbac.NewUserRoleMap(), nil)
	defer SetPolicy(nil, nil, nil)

	resolver := CohortResolver([]string{"sprout-1"})
	if resolver != nil {
		t.Error("CohortResolver should return nil when no registry loaded")
	}
}

func TestCohortResolverWithRegistry(t *testing.T) {
	reg := rbac.NewRegistry()
	reg.Register(&rbac.Cohort{
		Name:    "test-group",
		Type:    rbac.CohortTypeStatic,
		Members: []string{"s1", "s2"},
	})

	SetPolicy(rbac.NewRoleStore(), rbac.NewUserRoleMap(), reg)
	defer SetPolicy(nil, nil, nil)

	resolver := CohortResolver([]string{"s1", "s2", "s3"})
	if resolver == nil {
		t.Fatal("CohortResolver should not return nil when registry is loaded")
	}

	members, err := resolver("test-group")
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	if !members["s1"] || !members["s2"] {
		t.Error("expected s1 and s2 in resolved members")
	}
}

func TestListAllUsersWithPolicy(t *testing.T) {
	rs := rbac.NewRoleStore()
	rs.Register(&rbac.Role{
		Name:  "admin",
		Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
	})

	urm := rbac.NewUserRoleMap()
	urm.Set("AKEY1", "admin")
	urm.Set("AKEY2", "admin")

	SetPolicy(rs, urm, nil)
	defer SetPolicy(nil, nil, nil)

	users := ListAllUsers()
	if len(users) < 2 {
		t.Errorf("expected at least 2 users, got %d", len(users))
	}
	if users["AKEY1"] != "admin" {
		t.Errorf("AKEY1 role = %q, want 'admin'", users["AKEY1"])
	}
	if users["AKEY2"] != "admin" {
		t.Errorf("AKEY2 role = %q, want 'admin'", users["AKEY2"])
	}
}
