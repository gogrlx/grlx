package natsapi

import (
	"encoding/json"

	intauth "github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// natsActionMap maps NATS API method names to RBAC actions.
// Methods not listed here require ActionAdmin (deny by default).
var natsActionMap = map[string]rbac.Action{
	// Read-only
	"version":         rbac.ActionView,
	"sprouts.list":    rbac.ActionView,
	"sprouts.get":     rbac.ActionView,
	"jobs.list":       rbac.ActionView,
	"jobs.get":        rbac.ActionView,
	"jobs.forsprout":  rbac.ActionView,
	"props.getall":    rbac.ActionView,
	"props.get":       rbac.ActionView,
	"cohorts.list":    rbac.ActionView,
	"cohorts.resolve": rbac.ActionView,

	// Write: scoped
	"cook":         rbac.ActionCook,
	"cmd.run":      rbac.ActionCmd,
	"test.ping":    rbac.ActionTest,
	"props.set":    rbac.ActionProps,
	"props.delete": rbac.ActionProps,
	"jobs.cancel":  rbac.ActionJobAdmin,

	// Global: PKI
	"pki.list":     rbac.ActionPKI,
	"pki.accept":   rbac.ActionPKI,
	"pki.reject":   rbac.ActionPKI,
	"pki.deny":     rbac.ActionPKI,
	"pki.unaccept": rbac.ActionPKI,
	"pki.delete":   rbac.ActionPKI,

	// Auth
	"auth.whoami": rbac.ActionUserRead,
	"auth.users":  rbac.ActionAdmin,
}

// publicMethods are accessible without a token.
// version is informational; auth.whoami handles its own auth logic.
var publicMethods = map[string]bool{
	"version":     true,
	"auth.whoami": true,
}

// NATSMethodAction returns the RBAC action required for a NATS API method.
// Unknown methods require ActionAdmin (deny by default).
func NATSMethodAction(method string) rbac.Action {
	if a, ok := natsActionMap[method]; ok {
		return a
	}
	return rbac.ActionAdmin
}

// tokenParams is used to extract the auth token from NATS API params.
type tokenParams struct {
	Token string `json:"token"`
}

// authMiddleware wraps a handler with permission enforcement.
// It extracts the token from params, resolves the user's role, and
// checks whether the role permits the required action. Unauthorized
// requests are rejected before the handler runs.
//
// Public methods (version, auth.whoami) bypass the check entirely.
func authMiddleware(method string, next handler) handler {
	if publicMethods[method] {
		return next
	}

	requiredAction := NATSMethodAction(method)

	return func(params json.RawMessage) (any, error) {
		// dangerously_allow_root bypasses all auth checks.
		if intauth.DangerouslyAllowRoot() {
			return next(params)
		}

		// Extract token from params.
		var tp tokenParams
		if len(params) > 0 {
			json.Unmarshal(params, &tp)
		}
		if tp.Token == "" {
			return nil, rbac.ErrAccessDenied
		}

		// Check route-level access (does this role have the required action?).
		if !intauth.TokenHasAction(tp.Token, requiredAction) {
			return nil, rbac.ErrAccessDenied
		}

		return next(params)
	}
}
