package natsapi

import (
	"encoding/json"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/rbac"
)

func TestNATSMethodAction(t *testing.T) {
	tests := []struct {
		method string
		want   rbac.Action
	}{
		{"version", rbac.ActionView},
		{"sprouts.list", rbac.ActionView},
		{"sprouts.get", rbac.ActionView},
		{"cook", rbac.ActionCook},
		{"cmd.run", rbac.ActionCmd},
		{"test.ping", rbac.ActionTest},
		{"pki.accept", rbac.ActionPKI},
		{"pki.list", rbac.ActionPKI},
		{"props.set", rbac.ActionProps},
		{"props.delete", rbac.ActionProps},
		{"jobs.cancel", rbac.ActionJobAdmin},
		{"auth.whoami", rbac.ActionUserRead},
		{"auth.users", rbac.ActionAdmin},
		{"unknown.method", rbac.ActionAdmin},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := NATSMethodAction(tt.method)
			if got != tt.want {
				t.Errorf("NATSMethodAction(%q) = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

func TestPublicMethods(t *testing.T) {
	for method := range publicMethods {
		if !publicMethods[method] {
			t.Errorf("expected %q to be public", method)
		}
	}
	// Non-public methods should not be in the map.
	if publicMethods["cook"] {
		t.Error("cook should not be a public method")
	}
	if publicMethods["pki.accept"] {
		t.Error("pki.accept should not be a public method")
	}
}

func TestAuthMiddleware_PublicMethod(t *testing.T) {
	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "ok", nil
	}

	wrapped := authMiddleware("version", inner)
	result, err := wrapped(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("inner handler was not called for public method")
	}
	if result != "ok" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	inner := func(params json.RawMessage) (any, error) {
		t.Fatal("handler should not be called without a token")
		return nil, nil
	}

	wrapped := authMiddleware("cook", inner)

	// No params at all
	_, err := wrapped(nil)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if err != rbac.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got: %v", err)
	}

	// Empty params
	_, err = wrapped(json.RawMessage(`{}`))
	if err != rbac.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied for empty params, got: %v", err)
	}

	// Params with empty token
	_, err = wrapped(json.RawMessage(`{"token":""}`))
	if err != rbac.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied for empty token, got: %v", err)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	inner := func(params json.RawMessage) (any, error) {
		t.Fatal("handler should not be called with invalid token")
		return nil, nil
	}

	wrapped := authMiddleware("cook", inner)
	params := json.RawMessage(`{"token":"invalid-garbage-token"}`)
	_, err := wrapped(params)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if err != rbac.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestViewerRoleNATSAccess(t *testing.T) {
	viewer := rbac.BuiltinViewerRole()

	// Read-only NATS methods the viewer should access
	allowedMethods := []string{
		"version", "sprouts.list", "sprouts.get",
		"jobs.list", "jobs.get", "jobs.forsprout",
		"props.getall", "props.get",
		"cohorts.list", "cohorts.get", "cohorts.resolve", "cohorts.refresh",
		"auth.whoami", "auth.explain",
		"recipes.list", "recipes.get",
	}
	for _, method := range allowedMethods {
		action := NATSMethodAction(method)
		if !viewer.HasAction(action) {
			t.Errorf("viewer should be able to call %q (requires %q)", method, action)
		}
	}

	// Write NATS methods the viewer should be denied
	deniedMethods := []string{
		"cook", "cmd.run", "shell.start", "test.ping",
		"props.set", "props.delete",
		"jobs.cancel",
		"pki.list", "pki.accept", "pki.reject", "pki.deny", "pki.unaccept", "pki.delete",
		"auth.users", "auth.users.add", "auth.users.remove",
		"audit.dates", "audit.query",
	}
	for _, method := range deniedMethods {
		action := NATSMethodAction(method)
		if viewer.HasAction(action) {
			t.Errorf("viewer should NOT be able to call %q (requires %q)", method, action)
		}
	}
}

func TestOperatorRoleNATSAccess(t *testing.T) {
	op := rbac.BuiltinOperatorRole()

	// Methods the operator should access
	allowedMethods := []string{
		"version", "sprouts.list", "sprouts.get",
		"jobs.list", "jobs.get", "jobs.forsprout", "jobs.cancel",
		"props.getall", "props.get", "props.set", "props.delete",
		"cohorts.list", "cohorts.get", "cohorts.resolve", "cohorts.refresh",
		"cook", "cmd.run", "shell.start", "test.ping",
		"auth.whoami", "auth.explain",
		"recipes.list", "recipes.get",
	}
	for _, method := range allowedMethods {
		action := NATSMethodAction(method)
		if !op.HasAction(action) {
			t.Errorf("operator should be able to call %q (requires %q)", method, action)
		}
	}

	// Methods the operator should be denied (PKI + user management + audit)
	deniedMethods := []string{
		"pki.list", "pki.accept", "pki.reject", "pki.deny", "pki.unaccept", "pki.delete",
		"auth.users", "auth.users.add", "auth.users.remove",
		"audit.dates", "audit.query",
	}
	for _, method := range deniedMethods {
		action := NATSMethodAction(method)
		if op.HasAction(action) {
			t.Errorf("operator should NOT be able to call %q (requires %q)", method, action)
		}
	}
}

func TestAllRoutesHaveActionMapping(t *testing.T) {
	// Every route in the router should have an entry in natsActionMap.
	for method := range routes {
		if _, ok := natsActionMap[method]; !ok {
			t.Errorf("route %q has no entry in natsActionMap (will default to admin)", method)
		}
	}
}
