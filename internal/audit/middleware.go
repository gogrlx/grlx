package audit

import (
	"encoding/json"
	"sync"
)

// global logger, set during farmer startup.
var (
	globalMu     sync.RWMutex
	globalLogger *Logger
)

// SetGlobal sets the global audit logger used by the middleware.
func SetGlobal(l *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// Global returns the global audit logger, or nil if not configured.
func Global() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLogger
}

// readOnlyActions are actions that don't mutate state and are
// logged at a lower priority (still logged for completeness).
var readOnlyActions = map[string]bool{
	"version":        true,
	"sprouts.list":   true,
	"sprouts.get":    true,
	"jobs.list":      true,
	"jobs.get":       true,
	"jobs.forsprout": true,
	"props.getall":   true,
	"props.get":      true,
	"cohorts.list":   true,
	"cohorts.get":    true,
	"auth.whoami":    true,
	"auth.users":     true,
	"pki.list":       true,
}

// IsReadOnly returns true if the action is read-only.
func IsReadOnly(action string) bool {
	return readOnlyActions[action]
}

// LogAction is a convenience function that logs an action using the
// global logger. It extracts identity from the params if possible.
// If no global logger is set, it silently returns nil.
func LogAction(action string, params json.RawMessage, result any, err error) error {
	l := Global()
	if l == nil {
		return nil
	}

	// Extract identity from params (most NATS handlers pass a token).
	pubkey, roleName := extractIdentity(params)

	entry := Entry{
		Pubkey:   pubkey,
		RoleName: roleName,
		Action:   action,
		Success:  err == nil,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// For write actions, include the params for forensic value.
	if !IsReadOnly(action) && len(params) > 0 {
		// Redact the token from params before storing.
		entry.Parameters = redactToken(params)
	}

	// Extract targets from params if present.
	entry.Targets = extractTargets(params)

	return l.Log(entry)
}

// identityFields is used to extract pubkey/role from params.
type identityFields struct {
	Token string `json:"token"`
}

// extractIdentity attempts to resolve the user from the token in params.
// Returns empty strings if the token is missing or invalid.
func extractIdentity(params json.RawMessage) (pubkey, roleName string) {
	if len(params) == 0 {
		return "", ""
	}
	var fields identityFields
	if err := json.Unmarshal(params, &fields); err != nil || fields.Token == "" {
		return "", ""
	}

	// Use auth.WhoAmI to resolve — but we don't import auth here to
	// avoid circular dependencies. Instead, use the resolver function.
	resolverMu.RLock()
	resolve := identityResolver
	resolverMu.RUnlock()

	if resolve == nil {
		return "", ""
	}

	pk, role, err := resolve(fields.Token)
	if err != nil {
		return "", ""
	}
	return pk, role
}

// IdentityResolverFunc resolves a token to (pubkey, roleName, error).
type IdentityResolverFunc func(token string) (pubkey, roleName string, err error)

var (
	resolverMu       sync.RWMutex
	identityResolver IdentityResolverFunc
)

// SetIdentityResolver sets the function used to resolve tokens to identities.
// Typically set to auth.WhoAmI during farmer startup.
func SetIdentityResolver(fn IdentityResolverFunc) {
	resolverMu.Lock()
	defer resolverMu.Unlock()
	identityResolver = fn
}

// targetFields holds common fields used to extract sprout targets from params.
type targetFields struct {
	SproutID  string   `json:"sprout_id"`
	SproutIDs []string `json:"sprout_ids"`
	Target    []struct {
		SproutID string `json:"SproutID"`
	} `json:"target"`
}

// extractTargets attempts to find sprout IDs from common param fields.
func extractTargets(params json.RawMessage) []string {
	if len(params) == 0 {
		return nil
	}

	var fields targetFields
	if err := json.Unmarshal(params, &fields); err != nil {
		return nil
	}

	if len(fields.SproutIDs) > 0 {
		return fields.SproutIDs
	}
	if fields.SproutID != "" {
		return []string{fields.SproutID}
	}
	if len(fields.Target) > 0 {
		ids := make([]string, 0, len(fields.Target))
		for _, t := range fields.Target {
			if t.SproutID != "" {
				ids = append(ids, t.SproutID)
			}
		}
		if len(ids) > 0 {
			return ids
		}
	}

	return nil
}

// redactToken removes the "token" field from params to avoid storing
// sensitive credentials in the audit log.
func redactToken(params json.RawMessage) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(params, &m); err != nil {
		return params
	}
	if _, hasToken := m["token"]; !hasToken {
		return params
	}
	delete(m, "token")
	if len(m) == 0 {
		return nil
	}
	redacted, err := json.Marshal(m)
	if err != nil {
		return params
	}
	return redacted
}
