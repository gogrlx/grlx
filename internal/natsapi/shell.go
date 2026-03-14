package natsapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/audit"
	intauth "github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/log"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/shell"
)

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

	// Generate a unique session ID.
	sessionID := uuid.New().String()

	shellName := req.Shell
	if shellName == "" {
		shellName = "/bin/sh"
	}

	// Extract identity for audit logging.
	pubkey, roleName := extractShellIdentity(params)

	// Forward the start request to the sprout.
	startReq := shell.StartRequest{
		SessionID: sessionID,
		Cols:      req.Cols,
		Rows:      req.Rows,
		Shell:     req.Shell,
	}
	data, _ := json.Marshal(startReq)
	topic := "grlx.sprouts." + req.SproutID + ".shell.start"

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

	// Parse session subjects from the sprout response.
	var resp shell.StartResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("invalid sprout response: %w", err)
	}

	// Audit: log session start.
	if auditErr := audit.LogSessionStart(pubkey, roleName, sessionID, req.SproutID, shellName); auditErr != nil {
		log.Errorf("natsapi: audit session.start failed: %v", auditErr)
	}

	// Start background session tracker for byte counting and session.end audit.
	go trackSession(natsConn, sessionTrackInfo{
		pubkey:    pubkey,
		roleName:  roleName,
		sessionID: sessionID,
		sproutID:  req.SproutID,
		shell:     shellName,
		startTime: time.Now().UTC(),
		subjects:  resp,
	})

	return resp, nil
}

// sessionTrackInfo holds the info needed to track and audit a live session.
type sessionTrackInfo struct {
	pubkey    string
	roleName  string
	sessionID string
	sproutID  string
	shell     string
	startTime time.Time
	subjects  shell.StartResponse
}

// trackSession subscribes to the session's I/O subjects to count bytes
// and waits for the done signal to emit a session.end audit entry.
func trackSession(nc *nats.Conn, info sessionTrackInfo) {
	var bytesIn atomic.Int64
	var bytesOut atomic.Int64

	// Count bytes from CLI → sprout (input).
	inputSub, err := nc.Subscribe(info.subjects.InputSubject, func(msg *nats.Msg) {
		bytesIn.Add(int64(len(msg.Data)))
	})
	if err != nil {
		log.Errorf("natsapi: session %s: failed to track input: %v", info.sessionID, err)
	}

	// Count bytes from sprout → CLI (output).
	outputSub, err := nc.Subscribe(info.subjects.OutputSubject, func(msg *nats.Msg) {
		bytesOut.Add(int64(len(msg.Data)))
	})
	if err != nil {
		log.Errorf("natsapi: session %s: failed to track output: %v", info.sessionID, err)
	}

	// Wait for the done signal.
	doneCh := make(chan shell.DoneMessage, 1)
	doneSub, err := nc.Subscribe(info.subjects.DoneSubject, func(msg *nats.Msg) {
		var done shell.DoneMessage
		if jsonErr := json.Unmarshal(msg.Data, &done); jsonErr != nil {
			doneCh <- shell.DoneMessage{ExitCode: -1, Error: jsonErr.Error()}
		} else {
			doneCh <- done
		}
	})
	if err != nil {
		log.Errorf("natsapi: session %s: failed to track done: %v", info.sessionID, err)
		return
	}

	// Wait with a generous timeout (24h) to avoid leaking goroutines
	// if the done message is never received.
	const maxSessionDuration = 24 * time.Hour
	timer := time.NewTimer(maxSessionDuration)
	defer timer.Stop()

	var doneMsg shell.DoneMessage
	select {
	case doneMsg = <-doneCh:
		// Normal exit.
	case <-timer.C:
		doneMsg = shell.DoneMessage{ExitCode: -1, Error: "session timed out"}
		log.Warnf("natsapi: session %s timed out after %v", info.sessionID, maxSessionDuration)
	}

	// Clean up subscriptions.
	if inputSub != nil {
		inputSub.Unsubscribe()
	}
	if outputSub != nil {
		outputSub.Unsubscribe()
	}
	if doneSub != nil {
		doneSub.Unsubscribe()
	}

	// Audit: log session end.
	endInfo := audit.SessionEndInfo{
		Pubkey:    info.pubkey,
		RoleName:  info.roleName,
		SessionID: info.sessionID,
		SproutID:  info.sproutID,
		Shell:     info.shell,
		StartTime: info.startTime,
		BytesIn:   bytesIn.Load(),
		BytesOut:  bytesOut.Load(),
		ExitCode:  doneMsg.ExitCode,
		Error:     doneMsg.Error,
	}

	if auditErr := audit.LogSessionEnd(endInfo); auditErr != nil {
		log.Errorf("natsapi: audit session.end failed for %s: %v", info.sessionID, auditErr)
	}

	log.Debugf("natsapi: session %s ended (exit=%d, in=%d, out=%d, duration=%.1fs)",
		info.sessionID, doneMsg.ExitCode, bytesIn.Load(), bytesOut.Load(),
		time.Since(info.startTime).Seconds())
}

// extractShellIdentity resolves the user's pubkey and role from the
// auth token embedded in the shell start request params.
func extractShellIdentity(params json.RawMessage) (pubkey, roleName string) {
	if len(params) == 0 {
		return "", ""
	}

	var tp tokenParams
	if err := json.Unmarshal(params, &tp); err != nil || tp.Token == "" {
		return "", ""
	}

	pk, role, err := intauth.WhoAmI(tp.Token)
	if err != nil {
		return "", ""
	}
	return pk, role
}
