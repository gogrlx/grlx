package auth

import (
	"errors"

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
// Used by both the CLI and the sprout for mutual authentication.
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
// dangerously_allow_root set. This bypasses all auth checks and
// should only be used for development.
func DangerouslyAllowRoot() bool {
	return jety.GetBool("dangerously_allow_root")
}

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

// TokenHasRouteAccess checks whether the bearer token has permission to
// access the named route, using role-based access control. The routeName
// must match a key from the api.Routes map (e.g. "Cook", "AcceptID").
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
	role := pubkeyRole(pk)
	return rbac.RoleHasAccess(role, routeName)
}

// WhoAmI returns the public key and role for a given token.
// Returns empty strings if the token is invalid.
func WhoAmI(token string) (pubkey string, role rbac.Role, err error) {
	ua, err := decodeToken(token)
	if err != nil {
		return "", "", err
	}
	pk, err := ua.IsValid()
	if err != nil {
		return "", "", err
	}
	return pk, pubkeyRole(pk), nil
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

// pubkeyHasAccess is the legacy check — returns true if the pubkey has
// any recognized role. Kept for backward compatibility with TokenHasAccess.
func pubkeyHasAccess(pubkey string, method string) bool {
	role := pubkeyRole(pubkey)
	return role != ""
}

// pubkeyRole returns the role assigned to a pubkey. It checks the "users"
// config section first (new format), then falls back to the legacy "pubkeys"
// section where all keys are treated as admin. Returns empty string if not found.
func pubkeyRole(pubkey string) rbac.Role {
	// New format: users.<role> = [pubkey, ...]
	usersMap := jety.GetStringMap("users")
	if len(usersMap) > 0 {
		for roleName, v := range usersMap {
			role, err := rbac.ParseRole(roleName)
			if err != nil {
				continue
			}
			keys := extractStringSlice(v)
			for _, k := range keys {
				if k == pubkey {
					return role
				}
			}
		}
	}

	// Legacy format: pubkeys.<role> = [pubkey, ...]
	// All keys under "admin" are admin; other role names are checked too.
	pubkeysMap := jety.GetStringMap("pubkeys")
	for roleName, v := range pubkeysMap {
		role, err := rbac.ParseRole(roleName)
		if err != nil {
			// Legacy: unknown role names under pubkeys treated as admin
			role = rbac.RoleAdmin
		}
		keys := extractStringSlice(v)
		for _, k := range keys {
			if k == pubkey {
				return role
			}
		}
	}

	return ""
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

// ListUsers returns all configured users grouped by role.
// It checks both "users" and legacy "pubkeys" config sections.
func ListUsers() map[rbac.Role][]string {
	result := make(map[rbac.Role][]string)

	// New format first
	usersMap := jety.GetStringMap("users")
	for roleName, v := range usersMap {
		role, err := rbac.ParseRole(roleName)
		if err != nil {
			continue
		}
		keys := extractStringSlice(v)
		result[role] = append(result[role], keys...)
	}

	// Legacy format: add any keys not already present
	pubkeysMap := jety.GetStringMap("pubkeys")
	for roleName, v := range pubkeysMap {
		role, err := rbac.ParseRole(roleName)
		if err != nil {
			role = rbac.RoleAdmin
		}
		keys := extractStringSlice(v)
		for _, k := range keys {
			if !containsKey(result[role], k) {
				result[role] = append(result[role], k)
			}
		}
	}

	return result
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
