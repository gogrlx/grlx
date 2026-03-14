package rbac

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/taigrr/jety"
)

// ErrDuplicatePubkey is returned when the same pubkey appears under
// multiple roles in the users/pubkeys config sections.
var ErrDuplicatePubkey = errors.New("duplicate pubkey assignment")

// RoleStore holds all custom role definitions loaded from config.
type RoleStore struct {
	roles map[string]*Role
}

// NewRoleStore creates an empty role store.
func NewRoleStore() *RoleStore {
	return &RoleStore{roles: make(map[string]*Role)}
}

// Register adds a role to the store, replacing any existing role with
// the same name. Validates the role before registration.
func (rs *RoleStore) Register(r *Role) error {
	if err := r.Validate(); err != nil {
		return err
	}
	rs.roles[r.Name] = r
	return nil
}

// Get retrieves a role by name.
func (rs *RoleStore) Get(name string) (*Role, error) {
	r, ok := rs.roles[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRole, name)
	}
	return r, nil
}

// List returns all role names.
func (rs *RoleStore) List() []string {
	names := make([]string, 0, len(rs.roles))
	for name := range rs.roles {
		names = append(names, name)
	}
	return names
}

// BuiltinViewerRole returns the built-in "viewer" role with read-only
// permissions. This role grants view and user_read actions with wildcard
// scope — enough to list sprouts, view jobs/props/cohorts, and call
// whoami, but no write operations (cook, cmd, pki, etc.).
func BuiltinViewerRole() *Role {
	return &Role{
		Name: "viewer",
		Rules: []Rule{
			{Action: ActionView, Scope: "*"},
			{Action: ActionUserRead, Scope: "*"},
		},
	}
}

// LoadRolesFromConfig reads the "roles" section from the farmer config
// and returns a populated RoleStore. Returns an empty store if the
// section is missing.
//
// Built-in roles (currently just "viewer") are always registered unless
// the config defines a role with the same name, allowing admins to
// override built-in definitions.
//
// Expected config format:
//
//	roles:
//	  sre-team:
//	    - action: admin
//	  dev-team:
//	    - action: view
//	      scope: "*"
//	    - action: cook
//	      scope: "cohort:staging"
//	    - action: cmd
//	      scope: "cohort:dev"
//	  readonly:
//	    - action: view
//	    - action: user_read
func LoadRolesFromConfig() (*RoleStore, error) {
	store := NewRoleStore()

	// Register built-in roles first. Config-defined roles with the same
	// name will override these below.
	builtins := []*Role{BuiltinViewerRole()}
	for _, b := range builtins {
		_ = store.Register(b) // built-ins are always valid
	}

	raw := jety.GetStringMap("roles")
	if len(raw) == 0 {
		return store, nil
	}

	for name, v := range raw {
		role, err := parseRoleEntry(name, v)
		if err != nil {
			return nil, fmt.Errorf("parsing role %q: %w", name, err)
		}
		if err := store.Register(role); err != nil {
			return nil, fmt.Errorf("registering role %q: %w", name, err)
		}
	}

	return store, nil
}

func parseRoleEntry(name string, raw any) (*Role, error) {
	role := &Role{Name: name}

	rulesRaw, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("role %q: rules must be a list", name)
	}

	for i, ruleRaw := range rulesRaw {
		m, ok := ruleRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("role %q rule %d: must be a map", name, i)
		}

		actionStr, _ := m["action"].(string)
		action, err := ParseAction(strings.TrimSpace(actionStr))
		if err != nil {
			return nil, fmt.Errorf("role %q rule %d: %w", name, i, err)
		}

		scope, _ := m["scope"].(string)
		scope = strings.TrimSpace(scope)
		if scope == "" {
			scope = "*"
		}

		rule := Rule{Action: action, Scope: scope}
		role.Rules = append(role.Rules, rule)
	}

	return role, nil
}

// UserRoleMap maps public keys to role names. Loaded from the "users"
// section of the farmer config.
type UserRoleMap struct {
	pubkeyToRole map[string]string
}

// NewUserRoleMap creates an empty map.
func NewUserRoleMap() *UserRoleMap {
	return &UserRoleMap{pubkeyToRole: make(map[string]string)}
}

// Set assigns a role to a pubkey.
func (m *UserRoleMap) Set(pubkey, roleName string) {
	m.pubkeyToRole[pubkey] = roleName
}

// RoleName returns the role name for a pubkey, or empty string if not found.
func (m *UserRoleMap) RoleName(pubkey string) string {
	return m.pubkeyToRole[pubkey]
}

// All returns the full map of pubkey → role name.
func (m *UserRoleMap) All() map[string]string {
	result := make(map[string]string, len(m.pubkeyToRole))
	for k, v := range m.pubkeyToRole {
		result[k] = v
	}
	return result
}

// LoadUsersFromConfig reads the "users" section from the farmer config.
//
// Expected format:
//
//	users:
//	  sre-team:
//	    - APUBKEY1...
//	  dev-team:
//	    - APUBKEY2...
//
// Also reads legacy format:
//
//	pubkeys:
//	  admin:
//	    - APUBKEY1...
//
// Legacy pubkeys.admin maps to a built-in "admin" role if no explicit
// role definition exists (backward compatibility).
func LoadUsersFromConfig() *UserRoleMap {
	m := NewUserRoleMap()

	// New format: users.<role> = [pubkeys...]
	usersMap := jety.GetStringMap("users")
	for roleName, v := range usersMap {
		keys := parseStringSlice(v)
		for _, k := range keys {
			m.Set(k, roleName)
		}
	}

	// Legacy format: pubkeys.<role> = [pubkeys...]
	pubkeysMap := jety.GetStringMap("pubkeys")
	for roleName, v := range pubkeysMap {
		keys := parseStringSlice(v)
		for _, k := range keys {
			// Don't override if already set from users section.
			if m.RoleName(k) == "" {
				m.Set(k, roleName)
			}
		}
	}

	return m
}

// ValidateUserUniqueness checks the users and pubkeys config sections for
// pubkeys that appear under more than one role. Returns an error listing
// every duplicate pubkey and the roles it was assigned to. This catches
// misconfigurations that would silently overwrite role assignments.
func ValidateUserUniqueness() error {
	// Collect all pubkey → []role mappings from both config sections.
	seen := make(map[string][]string) // pubkey → list of role names

	usersMap := jety.GetStringMap("users")
	for roleName, v := range usersMap {
		keys := parseStringSlice(v)
		for _, k := range keys {
			seen[k] = append(seen[k], "users."+roleName)
		}
	}

	pubkeysMap := jety.GetStringMap("pubkeys")
	for roleName, v := range pubkeysMap {
		keys := parseStringSlice(v)
		for _, k := range keys {
			seen[k] = append(seen[k], "pubkeys."+roleName)
		}
	}

	// Find duplicates.
	var dupes []string
	for pubkey, roles := range seen {
		if len(roles) > 1 {
			sort.Strings(roles)
			// Truncate long pubkeys for readability.
			display := pubkey
			if len(display) > 16 {
				display = display[:16] + "..."
			}
			dupes = append(dupes, fmt.Sprintf("  %s → [%s]", display, strings.Join(roles, ", ")))
		}
	}
	if len(dupes) == 0 {
		return nil
	}

	sort.Strings(dupes)
	return fmt.Errorf("%w: the following pubkeys are assigned to multiple roles:\n%s",
		ErrDuplicatePubkey, strings.Join(dupes, "\n"))
}

// LoadCohortsFromConfig reads the "cohorts" section from the farmer config
// (via jety) and returns a populated Registry. It does not fail on an
// empty or missing cohorts section — it simply returns an empty registry.
func LoadCohortsFromConfig() (*Registry, error) {
	registry := NewRegistry()

	raw := jety.GetStringMap("cohorts")
	if len(raw) == 0 {
		return registry, nil
	}

	for name, v := range raw {
		cohort, err := parseCohortEntry(name, v)
		if err != nil {
			return nil, fmt.Errorf("parsing cohort %q: %w", name, err)
		}
		if err := registry.Register(cohort); err != nil {
			return nil, fmt.Errorf("registering cohort %q: %w", name, err)
		}
	}

	return registry, nil
}

func parseCohortEntry(name string, raw any) (*Cohort, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: cohort %q value is not a map", ErrInvalidCohort, name)
	}

	cohort := &Cohort{Name: name}

	typeStr, _ := m["type"].(string)
	switch CohortType(typeStr) {
	case CohortTypeStatic:
		cohort.Type = CohortTypeStatic
		cohort.Members = parseStringSlice(m["members"])
	case CohortTypeDynamic:
		cohort.Type = CohortTypeDynamic
		match, err := parseDynamicMatch(m["match"])
		if err != nil {
			return nil, fmt.Errorf("cohort %q: %w", name, err)
		}
		cohort.Match = match
	case CohortTypeCompound:
		cohort.Type = CohortTypeCompound
		compound, err := parseCompoundExpr(m["compound"])
		if err != nil {
			return nil, fmt.Errorf("cohort %q: %w", name, err)
		}
		cohort.Compound = compound
	default:
		return nil, fmt.Errorf("%w: unknown type %q for cohort %q", ErrInvalidCohort, typeStr, name)
	}

	return cohort, nil
}

func parseStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return s
	default:
		return nil
	}
}

func parseDynamicMatch(v any) (*DynamicMatch, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: dynamic match is nil", ErrInvalidCohort)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: match is not a map", ErrInvalidCohort)
	}
	propName, _ := m["prop_name"].(string)
	propValue, _ := m["prop_value"].(string)
	if propName == "" {
		return nil, fmt.Errorf("%w: match requires prop_name", ErrInvalidCohort)
	}
	return &DynamicMatch{
		PropName:  propName,
		PropValue: propValue,
	}, nil
}

func parseCompoundExpr(v any) (*CompoundExpr, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: compound expression is nil", ErrInvalidCohort)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: compound is not a map", ErrInvalidCohort)
	}
	opStr, _ := m["operator"].(string)
	op := Operator(opStr)
	if err := validateOperator(op); err != nil {
		return nil, err
	}
	operands := parseStringSlice(m["operands"])
	if len(operands) < 2 {
		return nil, fmt.Errorf("%w", ErrMissingOperands)
	}
	return &CompoundExpr{
		Operator: op,
		Operands: operands,
	}, nil
}
