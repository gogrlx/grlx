package natsapi

import (
	"encoding/json"

	intauth "github.com/gogrlx/grlx/v2/internal/auth"
	log "github.com/gogrlx/grlx/v2/internal/log"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// natsActionMap maps NATS API method names to RBAC actions.
// Methods not listed here require ActionAdmin (deny by default).
var natsActionMap = map[string]rbac.Action{
	// Read-only
	MethodHealth:          rbac.ActionView,
	MethodVersion:         rbac.ActionView,
	MethodSproutsList:     rbac.ActionView,
	MethodSproutsGet:      rbac.ActionView,
	MethodJobsList:        rbac.ActionView,
	MethodJobsGet:         rbac.ActionView,
	MethodJobsForSprout:   rbac.ActionView,
	MethodPropsGetAll:     rbac.ActionView,
	MethodPropsGet:        rbac.ActionView,
	MethodCohortsList:     rbac.ActionView,
	MethodCohortsGet:      rbac.ActionView,
	MethodCohortsResolve:  rbac.ActionView,
	MethodCohortsRefresh:  rbac.ActionView,
	MethodCohortsValidate: rbac.ActionView,

	// Write: scoped
	MethodCook:        rbac.ActionCook,
	MethodCmdRun:      rbac.ActionCmd,
	MethodShellStart:  rbac.ActionShell,
	MethodTestPing:    rbac.ActionTest,
	MethodPropsSet:    rbac.ActionProps,
	MethodPropsDelete: rbac.ActionProps,
	MethodJobsCancel:  rbac.ActionJobAdmin,
	MethodJobsDelete:  rbac.ActionJobAdmin,

	// Global: PKI
	MethodPKIList:     rbac.ActionPKI,
	MethodPKIAccept:   rbac.ActionPKI,
	MethodPKIReject:   rbac.ActionPKI,
	MethodPKIDeny:     rbac.ActionPKI,
	MethodPKIUnaccept: rbac.ActionPKI,
	MethodPKIDelete:   rbac.ActionPKI,

	// Auth
	MethodAuthWhoAmI:     rbac.ActionUserRead,
	MethodAuthListUsers:  rbac.ActionAdmin,
	MethodAuthAddUser:    rbac.ActionAdmin,
	MethodAuthRemoveUser: rbac.ActionAdmin,
	MethodAuthExplain:    rbac.ActionUserRead,

	// Recipes (read-only)
	MethodRecipesList: rbac.ActionView,
	MethodRecipesGet:  rbac.ActionView,

	// Audit
	MethodAuditDates: rbac.ActionAdmin,
	MethodAuditQuery: rbac.ActionAdmin,
}

// publicMethods are accessible without a token.
// version is informational; auth.whoami handles its own auth logic.
var publicMethods = map[string]bool{
	MethodHealth:      true,
	MethodVersion:     true,
	MethodAuthWhoAmI:  true,
	MethodAuthExplain: true,
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
// checks whether the role permits the required action. For methods
// that target specific sprouts, it also performs scope-level checks
// using the role's cohort/sprout scope rules.
//
// Public methods (version, auth.whoami) bypass the check entirely.
func authMiddleware(method string, next handler) handler {
	if publicMethods[method] {
		return next
	}

	requiredAction := NATSMethodAction(method)
	extractor := scopeExtractors[method]

	return func(params json.RawMessage) (any, error) {
		// dangerously_allow_root bypasses all auth checks.
		if intauth.DangerouslyAllowRoot() {
			log.Warnf("dangerously_allow_root: bypassing auth for NATS method %s", method)
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

		// Check scope-level access if this method targets specific sprouts.
		if extractor != nil {
			sproutIDs, err := extractor(params)
			if err != nil {
				// Extraction errors are not auth failures — let the
				// handler validate params and return a proper error.
				return next(params)
			}
			if err := checkScopedAccess(tp.Token, requiredAction, sproutIDs); err != nil {
				return nil, rbac.ErrAccessDenied
			}
		}

		return next(params)
	}
}
