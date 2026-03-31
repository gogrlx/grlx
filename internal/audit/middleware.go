package audit

import (
	"encoding/json"
	"sync"
)

// Level controls which actions are recorded in the audit log.
type Level string

const (
	// LevelAll logs every API action (read and write).
	LevelAll Level = "all"
	// LevelWrite logs only mutating (write) actions. This is the default.
	LevelWrite Level = "write"
	// LevelOff disables audit logging entirely.
	LevelOff Level = "off"
)

// ParseLevel converts a string to an audit Level. Unknown values
// default to LevelWrite for backward compatibility.
func ParseLevel(s string) Level {
	switch Level(s) {
	case LevelAll, LevelWrite, LevelOff:
		return Level(s)
	default:
		return LevelWrite
	}
}

// global logger, set during farmer startup.
var (
	globalMu     sync.RWMutex
	globalLogger *Logger
	globalLevel  Level = LevelWrite
)

// SetGlobal sets the global audit logger used by the middleware.
func SetGlobal(l *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// SetLevel sets the global audit level.
func SetLevel(level Level) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLevel = level
}

// GetLevel returns the current audit level.
func GetLevel() Level {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLevel
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
	"auth.login":     true,
	"auth.whoami":    true,
	"auth.users":     true,
	"pki.list":       true,
	"recipes.list":   true,
	"recipes.get":    true,
	"audit.dates":    true,
	"audit.query":    true,
}

// IsReadOnly returns true if the action is read-only.
func IsReadOnly(action string) bool {
	return readOnlyActions[action]
}

// ShouldLog returns true if the given action should be recorded at
// the current audit level.
func ShouldLog(action string) bool {
	level := GetLevel()
	switch level {
	case LevelOff:
		return false
	case LevelAll:
		return true
	default: // LevelWrite
		return !IsReadOnly(action)
	}
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
	pubkey, roleName, username := extractIdentity(params)

	entry := Entry{
		Username: username,
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
func extractIdentity(params json.RawMessage) (pubkey, roleName, username string) {
	if len(params) == 0 {
		return "", "", ""
	}
	var fields identityFields
	if err := json.Unmarshal(params, &fields); err != nil || fields.Token == "" {
		return "", "", ""
	}

	// Use auth.WhoAmI to resolve — but we don't import auth here to
	// avoid circular dependencies. Instead, use the resolver function.
	resolverMu.RLock()
	resolve := identityResolver
	resolverMu.RUnlock()

	if resolve == nil {
		return "", "", ""
	}

	pk, role, name, err := resolve(fields.Token)
	if err != nil {
		return "", "", ""
	}
	return pk, role, name
}

// IdentityResolverFunc resolves a token to (pubkey, roleName, username, error).
// Username is the human-readable label configured for the pubkey.
type IdentityResolverFunc func(token string) (pubkey, roleName, username string, err error)

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
