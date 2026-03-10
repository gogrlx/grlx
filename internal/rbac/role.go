// Package rbac provides role-based access control for grlx.
package rbac

import (
	"errors"
	"fmt"
)

// Role represents a named access level for grlx users.
type Role string

const (
	// RoleAdmin has full access to all API endpoints and PKI management.
	RoleAdmin Role = "admin"
	// RoleOperator can cook, run commands, manage sprouts, and view jobs
	// but cannot manage PKI keys or user configuration.
	RoleOperator Role = "operator"
	// RoleViewer has read-only access: version, sprout/job/prop/cohort listing,
	// and log viewing.
	RoleViewer Role = "viewer"
)

var (
	ErrUnknownRole   = errors.New("unknown role")
	ErrAccessDenied  = errors.New("access denied")
	ErrNoPubkeyMatch = errors.New("pubkey does not match any configured user")
)

// ValidRoles returns all recognized role names.
func ValidRoles() []Role {
	return []Role{RoleAdmin, RoleOperator, RoleViewer}
}

// IsValidRole checks whether a role string is recognized.
func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}

// routePermissions maps route names to the minimum role required.
// Routes not listed here are admin-only by default.
var routePermissions = map[string]Role{
	// Public (handled before auth middleware, listed here for completeness)
	"GetCertificate": RoleViewer,
	"PutNKey":        RoleViewer,

	// Read-only: viewer and above
	"GetVersion":        RoleViewer,
	"GetLogSocket":      RoleViewer,
	"GetLogPage":        RoleViewer,
	"ListCohorts":       RoleViewer,
	"ListSprouts":       RoleViewer,
	"GetSprout":         RoleViewer,
	"ListJobs":          RoleViewer,
	"GetJob":            RoleViewer,
	"ListJobsForSprout": RoleViewer,
	"GetAllProps":       RoleViewer,
	"GetProp":           RoleViewer,
	"ListID":            RoleViewer,

	// Operator: can cook, run commands, test, manage props, resolve cohorts
	"TestPing":      RoleOperator,
	"Cook":          RoleOperator,
	"CmdRun":        RoleOperator,
	"ResolveCohort": RoleOperator,
	"SetProp":       RoleOperator,
	"DeleteProp":    RoleOperator,
	"CancelJob":     RoleOperator,

	// Auth: whoami is available to all authenticated users, users list is admin-only
	"WhoAmI": RoleViewer,

	// Admin-only: PKI management, user listing
	"GetID":      RoleAdmin,
	"AcceptID":   RoleAdmin,
	"RejectID":   RoleAdmin,
	"DenyID":     RoleAdmin,
	"UnacceptID": RoleAdmin,
	"DeleteID":   RoleAdmin,
	"ListUsers":  RoleAdmin,
}

// roleRank returns a numeric rank for role comparison.
// Higher rank means more privileges.
func roleRank(r Role) int {
	switch r {
	case RoleViewer:
		return 1
	case RoleOperator:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}

// RoleHasAccess checks whether a role has permission to access a named route.
// Unknown routes default to admin-only.
func RoleHasAccess(userRole Role, routeName string) bool {
	required, ok := routePermissions[routeName]
	if !ok {
		// Unknown routes require admin.
		required = RoleAdmin
	}
	return roleRank(userRole) >= roleRank(required)
}

// RequiredRole returns the minimum role needed for a route.
func RequiredRole(routeName string) Role {
	if r, ok := routePermissions[routeName]; ok {
		return r
	}
	return RoleAdmin
}

// ParseRole converts a string to a Role, returning an error for unknown values.
func ParseRole(s string) (Role, error) {
	r := Role(s)
	if !IsValidRole(r) {
		return "", fmt.Errorf("%w: %q", ErrUnknownRole, s)
	}
	return r, nil
}
