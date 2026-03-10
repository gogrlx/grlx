package auth

import (
	"errors"
	"sync"

	"github.com/nats-io/nkeys"
	"github.com/taigrr/jety"

	"github.com/gogrlx/grlx/v2/internal/rbac"
)

var (
	ErrInvalidPubkey = errors.New("invalid pubkey format in config")
	ErrMissingAdmin  = errors.New("no admin pubkey found in config")
	ErrNoPrivkey     = errors.New("no private key found in config")
	ErrPrivkeyExists = errors.New("private key already exists in config")
	ErrNoPubkeys     = errors.New("no pubkeys found in config")
)

// policyState holds the loaded RBAC policy. It is populated by LoadPolicy
// during farmer startup and used by all auth checks.
var (
	policyMu    sync.RWMutex
	roleStore   *rbac.RoleStore
	userRoleMap *rbac.UserRoleMap
	cohortReg   *rbac.Registry
)

// LoadPolicy reads roles, users, and cohorts from the farmer config.
// It must be called during farmer startup before serving requests.
func LoadPolicy() error {
	policyMu.Lock()
	defer policyMu.Unlock()

	var err error
	roleStore, err = rbac.LoadRolesFromConfig()
	if err != nil {
		return err
	}
	userRoleMap = rbac.LoadUsersFromConfig()
	cohortReg, err = rbac.LoadCohortsFromConfig()
	if err != nil {
		return err
	}

	// If no roles defined but legacy pubkeys.admin exists, create a
	// built-in admin role so existing configs keep working.
	if len(roleStore.List()) == 0 {
		legacyKeys, legacyErr := GetPubkeysByRole("admin")
		if legacyErr == nil && len(legacyKeys) > 0 {
			adminRole := &rbac.Role{
				Name:  "admin",
				Rules: []rbac.Rule{{Action: rbac.ActionAdmin, Scope: "*"}},
			}
			roleStore.Register(adminRole)
			for _, k := range legacyKeys {
				if userRoleMap.RoleName(k) == "" {
					userRoleMap.Set(k, "admin")
				}
			}
		}
	}

	return nil
}

// SetPolicy sets the policy stores directly (for testing).
func SetPolicy(rs *rbac.RoleStore, urm *rbac.UserRoleMap, cr *rbac.Registry) {
	policyMu.Lock()
	defer policyMu.Unlock()
	roleStore = rs
	userRoleMap = urm
	cohortReg = cr
}

// CohortResolver returns a function that resolves a cohort name to its
// member sprouts, using the loaded cohort registry. Returns nil if no
// registry is loaded.
func CohortResolver(allSproutIDs []string) func(string) (map[string]bool, error) {
	policyMu.RLock()
	reg := cohortReg
	policyMu.RUnlock()

	if reg == nil {
		return nil
	}
	return func(name string) (map[string]bool, error) {
		return reg.Resolve(name, allSproutIDs)
	}
}

func GetPubkey() (string, error) {
	seed, err := getPrivateSeed()
	if err != nil {
		return "", err
	}
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return "", err
	}
	pubkey, err := kp.PublicKey()
	if err != nil {
		return "", err
	}
	return pubkey, nil
}

func CreatePrivkey() error {
	_, err := getPrivateSeed()
	if !errors.Is(err, ErrNoPrivkey) {
		return ErrPrivkeyExists
	}
	_, err = createPrivateSeed()
	return err
}

func getPrivateSeed() (string, error) {
	seed := jety.GetString("privkey")
	if seed == "" {
		return "", ErrNoPrivkey
	}
	return seed, nil
}

func NewToken() (string, error) {
	seed, err := getPrivateSeed()
	if err != nil {
		return "", err
	}
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return "", err
	}
	return createSignedToken(kp)
}

// Sign signs a nonce using the local private key.
func Sign(nonce []byte) ([]byte, error) {
	seed, err := getPrivateSeed()
	if err != nil {
		return nil, err
	}
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, err
	}
	b, err := kp.Sign(nonce)
	kp.Wipe()
	return b, err
}

// DangerouslyAllowRoot returns true if the farmer config has
// dangerously_allow_root set. Bypasses all auth checks (dev only).
func DangerouslyAllowRoot() bool {
	return jety.GetBool("dangerously_allow_root")
}

// TokenHasAccess is the legacy auth check — returns true if the token
// maps to any configured user. Kept for backward compatibility.
func TokenHasAccess(token string, method string) bool {
	if DangerouslyAllowRoot() {
		return true
	}
	ua, err := decodeToken(token)
	if err != nil {
		return false
	}
	pk, err := ua.IsValid()
	if err != nil {
		return false
	}
	return pubkeyHasAccess(pk, method)
}

// TokenHasRouteAccess checks whether the bearer token has permission
// for the named route (e.g. "Cook", "AcceptID"). Uses policy-based RBAC.
func TokenHasRouteAccess(token string, routeName string) bool {
	if DangerouslyAllowRoot() {
		return true
	}
	ua, err := decodeToken(token)
	if err != nil {
		return false
	}
	pk, err := ua.IsValid()
	if err != nil {
		return false
	}
	role := lookupRole(pk)
	if role == nil {
		return false
	}
	return role.HasRouteAccess(routeName)
}

// TokenHasScopedAccess checks whether the token has permission for a
// specific action on specific sprout IDs. Used by handlers that need
// scope-level checks (cook, cmd, props, etc.).
func TokenHasScopedAccess(token string, action rbac.Action, sproutIDs []string, allSproutIDs []string) bool {
	if DangerouslyAllowRoot() {
		return true
	}
	ua, err := decodeToken(token)
	if err != nil {
		return false
	}
	pk, err := ua.IsValid()
	if err != nil {
		return false
	}
	role := lookupRole(pk)
	if role == nil {
		return false
	}
	resolver := CohortResolver(allSproutIDs)
	return role.HasScopedAccessMulti(action, sproutIDs, resolver)
}

// TokenScopeFilter returns the subset of sproutIDs that the token's role
// permits for the given action.
func TokenScopeFilter(token string, action rbac.Action, sproutIDs []string, allSproutIDs []string) []string {
	if DangerouslyAllowRoot() {
		return sproutIDs
	}
	ua, err := decodeToken(token)
	if err != nil {
		return nil
	}
	pk, err := ua.IsValid()
	if err != nil {
		return nil
	}
	role := lookupRole(pk)
	if role == nil {
		return nil
	}
	resolver := CohortResolver(allSproutIDs)
	return role.ScopeFilter(action, sproutIDs, resolver)
}

// WhoAmI returns the public key and role name for a given token.
func WhoAmI(token string) (pubkey string, roleName string, err error) {
	ua, err := decodeToken(token)
	if err != nil {
		return "", "", err
	}
	pk, err := ua.IsValid()
	if err != nil {
		return "", "", err
	}

	policyMu.RLock()
	defer policyMu.RUnlock()
	name := ""
	if userRoleMap != nil {
		name = userRoleMap.RoleName(pk)
	}
	if name == "" {
		// Check legacy
		name = legacyRoleName(pk)
	}

	return pk, name, nil
}

// lookupRole returns the Role object for a pubkey, or nil if not found.
func lookupRole(pubkey string) *rbac.Role {
	policyMu.RLock()
	defer policyMu.RUnlock()

	if userRoleMap == nil || roleStore == nil {
		return nil
	}

	name := userRoleMap.RoleName(pubkey)
	if name == "" {
		name = legacyRoleName(pubkey)
	}
	if name == "" {
		return nil
	}

	role, err := roleStore.Get(name)
	if err != nil {
		return nil
	}
	return role
}

// legacyRoleName checks the legacy pubkeys config section.
// Must be called with policyMu held (at least read).
func legacyRoleName(pubkey string) string {
	pubkeysMap := jety.GetStringMap("pubkeys")
	for roleName, v := range pubkeysMap {
		keys := extractStringSlice(v)
		for _, k := range keys {
			if k == pubkey {
				return roleName
			}
		}
	}
	return ""
}

// pubkeyHasAccess is the legacy check — returns true if the pubkey
// maps to any role. Kept for backward compatibility with TokenHasAccess.
func pubkeyHasAccess(pubkey string, method string) bool {
	return lookupRole(pubkey) != nil
}

// extractStringSlice handles both []any and []string from config values.
func extractStringSlice(v any) []string {
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
	case string:
		return []string{s}
	default:
		return nil
	}
}

// ListAllUsers returns all configured users as a map of pubkey → role name.
func ListAllUsers() map[string]string {
	policyMu.RLock()
	defer policyMu.RUnlock()

	result := make(map[string]string)
	if userRoleMap != nil {
		for k, v := range userRoleMap.All() {
			result[k] = v
		}
	}

	// Add legacy users not already present
	pubkeysMap := jety.GetStringMap("pubkeys")
	for roleName, v := range pubkeysMap {
		keys := extractStringSlice(v)
		for _, k := range keys {
			if _, exists := result[k]; !exists {
				result[k] = roleName
			}
		}
	}

	return result
}

// ListRoles returns all configured role names.
func ListRoles() []string {
	policyMu.RLock()
	defer policyMu.RUnlock()
	if roleStore == nil {
		return nil
	}
	return roleStore.List()
}

// GetRole returns the full role definition by name.
func GetRole(name string) (*rbac.Role, error) {
	policyMu.RLock()
	defer policyMu.RUnlock()
	if roleStore == nil {
		return nil, rbac.ErrUnknownRole
	}
	return roleStore.Get(name)
}

func GetPubkeysByRole(role string) ([]string, error) {
	err := jety.ReadInConfig()
	if err != nil {
		return []string{}, err
	}
	authKeySet := jety.GetStringMap("pubkeys")
	if len(authKeySet) == 0 {
		return []string{}, ErrNoPubkeys
	}
	i, ok := authKeySet[role]
	if !ok {
		return []string{}, ErrMissingAdmin
	}
	keys := []string{}
	if adminKey, ok := i.(string); !ok {
		if adminKeyList, ok := i.([]interface{}); ok {
			for _, k := range adminKeyList {
				if str, ok := k.(string); ok {
					keys = append(keys, str)
				} else {
					return []string{}, ErrInvalidPubkey
				}
			}
			return keys, nil
		} else {
			return []string{}, ErrInvalidPubkey
		}
	} else {
		return []string{adminKey}, nil
	}
}

func containsKey(slice []string, key string) bool {
	for _, s := range slice {
		if s == key {
			return true
		}
	}
	return false
}

func createPrivateSeed() (string, error) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		return "", err
	}
	seed, err := kp.Seed()
	if err != nil {
		return "", err
	}
	jety.Set("privkey", string(seed))
	jety.WriteConfig()
	return string(seed), nil
}
