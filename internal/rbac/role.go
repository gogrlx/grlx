// Package rbac provides role-based access control for grlx.
//
// Permissions are defined as (action, scope) pairs. Actions describe what
// a user can do (view, cook, cmd, pki, etc.) and scopes restrict which
// sprouts or cohorts the action applies to.
//
// Roles are named collections of permission rules defined in the farmer
// config. There are no built-in role names — admins define them freely.
package rbac

import (
	"errors"
	"fmt"
	"strings"
)

// Action represents a category of API operations.
type Action string

const (
	// ActionAdmin grants all permissions (superuser). Scope is ignored.
	ActionAdmin Action = "admin"

	// Scoped actions — these respect cohort/sprout scope.
	ActionView     Action = "view"      // read sprouts, jobs, props, cohorts, logs
	ActionCook     Action = "cook"      // apply recipes
	ActionCmd      Action = "cmd"       // run arbitrary commands
	ActionTest     Action = "test"      // test ping
	ActionProps    Action = "props"     // read/write props
	ActionJobAdmin Action = "job_admin" // cancel jobs

	// Global actions — these are not scoped to cohorts.
	ActionPKI      Action = "pki"       // accept/reject/deny/delete keys
	ActionUserRead Action = "user_read" // whoami
)

// AllActions returns every recognized action.
func AllActions() []Action {
	return []Action{
		ActionAdmin,
		ActionView, ActionCook, ActionCmd, ActionTest,
		ActionProps, ActionJobAdmin,
		ActionPKI, ActionUserRead,
	}
}

// IsValidAction checks whether an action string is recognized.
func IsValidAction(a Action) bool {
	for _, valid := range AllActions() {
		if a == valid {
			return true
		}
	}
	return false
}

var (
	ErrUnknownRole   = errors.New("unknown role")
	ErrAccessDenied  = errors.New("access denied")
	ErrNoPubkeyMatch = errors.New("pubkey does not match any configured user")
	ErrUnknownAction = errors.New("unknown action")
)

// Rule is a single permission entry: an action plus a scope.
// Scope can be:
//   - "*"           — all sprouts/cohorts (or global for unscoped actions)
//   - "cohort:web"  — members of cohort "web"
//   - "sprout:db-1" — a specific sprout
//
// An empty scope is treated as "*" for convenience.
type Rule struct {
	Action Action `json:"action" yaml:"action"`
	Scope  string `json:"scope,omitempty" yaml:"scope,omitempty"`
}

// Role is a named collection of permission rules.
type Role struct {
	Name  string `json:"name" yaml:"name"`
	Rules []Rule `json:"rules" yaml:"rules"`
}

// routeActions maps route names to the action(s) they require.
// A route may require multiple actions (e.g., Cook requires both "cook"
// and implicitly "view" for the targeted sprouts).
var routeActions = map[string]Action{
	// Public (handled before auth middleware)
	"GetCertificate": ActionView,
	"PutNKey":        ActionView,

	// Read-only
	"GetVersion":        ActionView,
	"GetLogSocket":      ActionView,
	"GetLogPage":        ActionView,
	"ListCohorts":       ActionView,
	"ListSprouts":       ActionView,
	"GetSprout":         ActionView,
	"ListJobs":          ActionView,
	"GetJob":            ActionView,
	"ListJobsForSprout": ActionView,
	"GetAllProps":       ActionView,
	"GetProp":           ActionView,
	"ListID":            ActionView,

	// Scoped write operations
	"TestPing":      ActionTest,
	"Cook":          ActionCook,
	"CmdRun":        ActionCmd,
	"ResolveCohort": ActionView,
	"SetProp":       ActionProps,
	"DeleteProp":    ActionProps,
	"CancelJob":     ActionJobAdmin,

	// Global: PKI management
	"GetID":      ActionPKI,
	"AcceptID":   ActionPKI,
	"RejectID":   ActionPKI,
	"DenyID":     ActionPKI,
	"UnacceptID": ActionPKI,
	"DeleteID":   ActionPKI,

	// Auth
	"WhoAmI":    ActionUserRead,
	"ListUsers": ActionAdmin,
}

// RouteAction returns the action required for a named API route.
// Unknown routes require ActionAdmin.
func RouteAction(routeName string) Action {
	if a, ok := routeActions[routeName]; ok {
		return a
	}
	return ActionAdmin
}

// HasRouteAccess checks whether a role has permission for a route,
// without scope checking. Use HasScopedAccess for scoped checks.
func (r *Role) HasRouteAccess(routeName string) bool {
	action := RouteAction(routeName)
	return r.HasAction(action)
}

// HasAction checks whether the role has any rule granting the given action.
func (r *Role) HasAction(action Action) bool {
	for _, rule := range r.Rules {
		if rule.Action == ActionAdmin {
			return true
		}
		if rule.Action == action {
			return true
		}
	}
	return false
}

// HasScopedAccess checks whether the role has permission for a given action
// on a specific sprout. The resolver function maps cohort names to sprout
// sets (used when a rule's scope is "cohort:X").
//
// If resolver is nil, cohort-scoped rules are skipped (only wildcard and
// sprout-specific rules are checked).
func (r *Role) HasScopedAccess(action Action, sproutID string, resolver func(cohortName string) (map[string]bool, error)) bool {
	for _, rule := range r.Rules {
		if rule.Action != ActionAdmin && rule.Action != action {
			continue
		}

		scope := rule.Scope
		if scope == "" || scope == "*" {
			return true
		}

		if strings.HasPrefix(scope, "sprout:") {
			target := scope[len("sprout:"):]
			if target == sproutID {
				return true
			}
			continue
		}

		if strings.HasPrefix(scope, "cohort:") {
			cohortName := scope[len("cohort:"):]
			if resolver == nil {
				continue
			}
			members, err := resolver(cohortName)
			if err != nil {
				continue
			}
			if members[sproutID] {
				return true
			}
			continue
		}

		// Unknown scope format — treat as no match.
	}
	return false
}

// HasScopedAccessMulti checks whether the role has permission for an action
// on ALL of the given sprout IDs. Returns false if any sprout is denied.
func (r *Role) HasScopedAccessMulti(action Action, sproutIDs []string, resolver func(cohortName string) (map[string]bool, error)) bool {
	for _, id := range sproutIDs {
		if !r.HasScopedAccess(action, id, resolver) {
			return false
		}
	}
	return true
}

// ScopeFilter returns the subset of sproutIDs that this role can access
// for the given action.
func (r *Role) ScopeFilter(action Action, sproutIDs []string, resolver func(cohortName string) (map[string]bool, error)) []string {
	var allowed []string
	for _, id := range sproutIDs {
		if r.HasScopedAccess(action, id, resolver) {
			allowed = append(allowed, id)
		}
	}
	return allowed
}

// Validate checks that all rules in the role reference valid actions and
// have parseable scopes.
func (r *Role) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("role name is required")
	}
	for i, rule := range r.Rules {
		if !IsValidAction(rule.Action) {
			return fmt.Errorf("rule %d: %w: %q", i, ErrUnknownAction, rule.Action)
		}
		if err := validateScope(rule.Scope); err != nil {
			return fmt.Errorf("rule %d: %w", i, err)
		}
	}
	return nil
}

func validateScope(scope string) error {
	if scope == "" || scope == "*" {
		return nil
	}
	if strings.HasPrefix(scope, "sprout:") {
		if scope == "sprout:" {
			return fmt.Errorf("sprout scope requires an ID")
		}
		return nil
	}
	if strings.HasPrefix(scope, "cohort:") {
		if scope == "cohort:" {
			return fmt.Errorf("cohort scope requires a name")
		}
		return nil
	}
	return fmt.Errorf("unknown scope format: %q (expected *, sprout:<id>, or cohort:<name>)", scope)
}

// ParseAction converts a string to an Action.
func ParseAction(s string) (Action, error) {
	a := Action(s)
	if !IsValidAction(a) {
		return "", fmt.Errorf("%w: %q", ErrUnknownAction, s)
	}
	return a, nil
}
