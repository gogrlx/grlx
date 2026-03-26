package natsapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/audit"
	intauth "github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/shell"
)

// sessionTracker tracks active shell sessions on the farmer for audit logging.
var sessionTracker = shell.NewTracker()

func handleShellStart(params json.RawMessage) (any, error) {
	var req shell.CLIStartRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if req.SproutID == "" {
		return nil, fmt.Errorf("sprout_id is required")
	}

	// Validate the sprout exists and is accepted.
	if !pki.IsValidSproutID(req.SproutID) || strings.Contains(req.SproutID, "_") {
		return nil, fmt.Errorf("invalid sprout ID: %s", req.SproutID)
	}
	registered, _ := pki.NKeyExists(req.SproutID, "")
	if !registered {
		return nil, fmt.Errorf("unknown sprout: %s", req.SproutID)
	}

	// Resolve the caller's identity for audit logging.
	pubkey, roleName := resolveCallerIdentity(params)

	// Generate a unique session ID.
	sessionID := uuid.New().String()

	// Forward the start request to the sprout.
	startReq := shell.StartRequest{
		SessionID:      sessionID,
		Cols:           req.Cols,
		Rows:           req.Rows,
		Shell:          req.Shell,
		IdleTimeoutSec: req.IdleTimeoutSec,
	}
	data, _ := json.Marshal(startReq)
	topic := SproutSubject(req.SproutID, SproutShellStart)

	msg, err := natsConn.Request(topic, data, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("sprout did not respond: %w", err)
	}

	// Check if sprout returned an error.
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(msg.Data, &errResp) == nil && errResp.Error != "" {
		return nil, fmt.Errorf("sprout error: %s", errResp.Error)
	}

	// Return session subjects to the CLI.
	var resp shell.StartResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("invalid sprout response: %w", err)
	}

	// Note: the shell.start audit entry (success or failure) is logged
	// by the generic audit middleware in router.go. The handler only
	// explicitly logs the shell.end event below, which the router
	// cannot track since it happens asynchronously via NATS.

	// Track the session and subscribe to its done subject for end logging.
	sessionInfo := &shell.SessionInfo{
		SessionID:   sessionID,
		SproutID:    req.SproutID,
		Pubkey:      pubkey,
		RoleName:    roleName,
		Shell:       req.Shell,
		StartedAt:   time.Now().UTC(),
		DoneSubject: resp.DoneSubject,
	}
	sessionTracker.Add(sessionInfo)
	subscribeSessionDone(sessionInfo)

	log.Infof("shell: session %s started (user=%s, sprout=%s)", sessionID, pubkey, req.SproutID)

	return resp, nil
}

// resolveCallerIdentity extracts the pubkey and role from the token in params.
func resolveCallerIdentity(params json.RawMessage) (pubkey, roleName string) {
	var tp struct {
		Token string `json:"token"`
	}
	if len(params) == 0 {
		return "", ""
	}
	if err := json.Unmarshal(params, &tp); err != nil || tp.Token == "" {
		return "", ""
	}
	pk, role, err := intauth.WhoAmI(tp.Token)
	if err != nil {
		return "", ""
	}
	return pk, role
}

// subscribeSessionDone subscribes to the session's done subject on the farmer
// side. When the sprout publishes the done message, the farmer logs the
// session end with duration.
func subscribeSessionDone(info *shell.SessionInfo) {
	if natsConn == nil || info.DoneSubject == "" {
		return
	}

	sub, err := natsConn.Subscribe(info.DoneSubject, func(msg *nats.Msg) {
		tracked := sessionTracker.Remove(info.SessionID)
		if tracked == nil {
			return
		}

		duration := time.Since(tracked.StartedAt)

		var done shell.DoneMessage
		json.Unmarshal(msg.Data, &done)

		errMsg := ""
		if done.Error != "" {
			errMsg = done.Error
		}

		logShellEnd(tracked, duration, done.ExitCode, errMsg)
		log.Infof("shell: session %s ended (user=%s, sprout=%s, duration=%s, exit=%d)",
			tracked.SessionID, tracked.Pubkey, tracked.SproutID,
			duration.Round(time.Second), done.ExitCode)
	})
	if err != nil {
		log.Errorf("shell: failed to subscribe to done subject for session %s: %v", info.SessionID, err)
		return
	}

	// Auto-unsubscribe after receiving one message.
	sub.AutoUnsubscribe(1)
}

// logShellEnd logs the session end event with duration and exit code.
func logShellEnd(info *shell.SessionInfo, duration time.Duration, exitCode int, errMsg string) {
	logger := audit.Global()
	if logger == nil {
		return
	}

	params := map[string]any{
		"session_id":   info.SessionID,
		"sprout_id":    info.SproutID,
		"duration_sec": duration.Seconds(),
		"exit_code":    exitCode,
	}
	if info.Shell != "" {
		params["shell"] = info.Shell
	}
	paramsJSON, _ := json.Marshal(params)

	entry := audit.Entry{
		Timestamp:  time.Now().UTC(),
		Pubkey:     info.Pubkey,
		RoleName:   info.RoleName,
		Action:     "shell.end",
		Targets:    []string{info.SproutID},
		Parameters: paramsJSON,
		Success:    errMsg == "",
		Error:      errMsg,
	}

	if err := logger.Log(entry); err != nil {
		log.Errorf("shell audit: failed to log shell.end: %v", err)
	}
}

// ShellTracker returns the farmer-side session tracker for shell sessions.
// This is useful for listing active sessions or checking session counts.
func ShellTracker() *shell.Tracker {
	return sessionTracker
}
