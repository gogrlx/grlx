package rbac

import (
	"fmt"
	"sort"
	"strings"
)

// Policy bundles a RoleStore, UserRoleMap, and cohort Registry into a
// single validated unit. Use ValidatePolicy to check for misconfigurations
// such as users referencing nonexistent roles or scopes referencing
// nonexistent cohorts.
type Policy struct {
	Roles   *RoleStore
	Users   *UserRoleMap
	Cohorts *Registry
}

// PolicyWarning describes a non-fatal misconfiguration.
type PolicyWarning struct {
	Kind    string `json:"kind"`    // "orphan_role_ref", "orphan_cohort_ref", "empty_role", etc.
	Message string `json:"message"` // human-readable description
}

// ValidatePolicy checks the policy for misconfigurations and returns
// warnings. It does not return an error for issues that can be
// tolerated at runtime (e.g., a user referencing a role that doesn't
// exist simply means they'll be denied access). Fatal errors during
// parsing are caught earlier in LoadRolesFromConfig/LoadUsersFromConfig.
func ValidatePolicy(p *Policy) []PolicyWarning {
	var warnings []PolicyWarning

	if p.Roles == nil || p.Users == nil {
		return warnings
	}

	// Check: every user references a role that exists.
	for pubkey, roleName := range p.Users.All() {
		if _, err := p.Roles.Get(roleName); err != nil {
			warnings = append(warnings, PolicyWarning{
				Kind:    "orphan_role_ref",
				Message: fmt.Sprintf("user %s references role %q which is not defined", truncatePubkey(pubkey), roleName),
			})
		}
	}

	// Check: every role's cohort-scoped rules reference cohorts that exist.
	if p.Cohorts != nil {
		for _, roleName := range p.Roles.List() {
			role, _ := p.Roles.Get(roleName)
			for _, rule := range role.Rules {
				if strings.HasPrefix(rule.Scope, "cohort:") {
					cohortName := rule.Scope[len("cohort:"):]
					if _, err := p.Cohorts.Get(cohortName); err != nil {
						warnings = append(warnings, PolicyWarning{
							Kind:    "orphan_cohort_ref",
							Message: fmt.Sprintf("role %q rule (%s → %s) references cohort %q which is not defined", roleName, rule.Action, rule.Scope, cohortName),
						})
					}
				}
			}
		}
	}

	// Check: roles with no rules (effectively deny-all).
	for _, roleName := range p.Roles.List() {
		role, _ := p.Roles.Get(roleName)
		if len(role.Rules) == 0 {
			warnings = append(warnings, PolicyWarning{
				Kind:    "empty_role",
				Message: fmt.Sprintf("role %q has no rules (all actions will be denied)", roleName),
			})
		}
	}

	// Check: roles that no user references (dead config).
	referenced := make(map[string]bool)
	for _, roleName := range p.Users.allRoleNames() {
		referenced[roleName] = true
	}
	for _, roleName := range p.Roles.List() {
		if !referenced[roleName] {
			warnings = append(warnings, PolicyWarning{
				Kind:    "unused_role",
				Message: fmt.Sprintf("role %q is defined but no user references it", roleName),
			})
		}
	}

	return warnings
}

// PermissionSummary describes what a single user can do.
type PermissionSummary struct {
	Pubkey   string          `json:"pubkey"`
	RoleName string          `json:"role"`
	IsAdmin  bool            `json:"isAdmin"`
	Actions  []ActionSummary `json:"actions"`
	Warnings []PolicyWarning `json:"warnings,omitempty"`
}

// ActionSummary describes one permitted action and its scope.
type ActionSummary struct {
	Action Action `json:"action"`
	Scope  string `json:"scope"`
}

// ExplainAccess returns a structured summary of what a specific user
// (identified by pubkey) is allowed to do under this policy.
func ExplainAccess(p *Policy, pubkey string) PermissionSummary {
	summary := PermissionSummary{Pubkey: pubkey}

	if p.Users == nil || p.Roles == nil {
		summary.Warnings = append(summary.Warnings, PolicyWarning{
			Kind:    "no_policy",
			Message: "RBAC policy is not loaded",
		})
		return summary
	}

	roleName := p.Users.RoleName(pubkey)
	if roleName == "" {
		summary.Warnings = append(summary.Warnings, PolicyWarning{
			Kind:    "no_role",
			Message: "pubkey is not assigned to any role",
		})
		return summary
	}
	summary.RoleName = roleName

	role, err := p.Roles.Get(roleName)
	if err != nil {
		summary.Warnings = append(summary.Warnings, PolicyWarning{
			Kind:    "orphan_role_ref",
			Message: fmt.Sprintf("assigned role %q does not exist", roleName),
		})
		return summary
	}

	for _, rule := range role.Rules {
		if rule.Action == ActionAdmin {
			summary.IsAdmin = true
		}
		scope := rule.Scope
		if scope == "" {
			scope = "*"
		}
		summary.Actions = append(summary.Actions, ActionSummary{
			Action: rule.Action,
			Scope:  scope,
		})
	}

	return summary
}

// ExplainAllUsers returns permission summaries for every configured user.
func ExplainAllUsers(p *Policy) []PermissionSummary {
	if p.Users == nil {
		return nil
	}

	var summaries []PermissionSummary
	allUsers := p.Users.All()

	// Sort by pubkey for deterministic output.
	keys := make([]string, 0, len(allUsers))
	for k := range allUsers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, pubkey := range keys {
		summaries = append(summaries, ExplainAccess(p, pubkey))
	}
	return summaries
}

// truncatePubkey shortens a pubkey for display in warnings.
func truncatePubkey(pk string) string {
	if len(pk) <= 12 {
		return pk
	}
	return pk[:6] + "..." + pk[len(pk)-6:]
}

// allRoleNames returns all unique role names referenced by users.
func (m *UserRoleMap) allRoleNames() []string {
	seen := make(map[string]bool)
	for _, name := range m.pubkeyToRole {
		seen[name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}
