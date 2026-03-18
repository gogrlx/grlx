// Package shell provides types and helpers for interactive shell sessions
// over NATS. A shell session allows the CLI user to get a remote terminal
// on a sprout, relayed entirely through the NATS bus.
package shell

import "fmt"

// DefaultIdleTimeout is the default duration before an idle shell session
// is terminated. Zero means no timeout.
const DefaultIdleTimeout = 0

// StartRequest is sent from the CLI (via the farmer) to a sprout to
// begin an interactive shell session.
type StartRequest struct {
	SessionID      string `json:"session_id"`
	Cols           int    `json:"cols"`
	Rows           int    `json:"rows"`
	Shell          string `json:"shell,omitempty"`            // default: /bin/sh
	IdleTimeoutSec int    `json:"idle_timeout_sec,omitempty"` // 0 = no timeout
}

// StartResponse is returned to the CLI after the sprout successfully
// starts the shell process.
type StartResponse struct {
	SessionID     string `json:"session_id"`
	InputSubject  string `json:"input_subject"`
	OutputSubject string `json:"output_subject"`
	ResizeSubject string `json:"resize_subject"`
	DoneSubject   string `json:"done_subject"`
}

// ResizeMessage is sent from the CLI to the sprout when the terminal
// window size changes.
type ResizeMessage struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// DoneMessage is published by the sprout when the shell process exits.
type DoneMessage struct {
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// CLIStartRequest is what the CLI sends to the farmer's grlx.api.shell.start.
type CLIStartRequest struct {
	SproutID       string `json:"sprout_id"`
	Cols           int    `json:"cols"`
	Rows           int    `json:"rows"`
	Shell          string `json:"shell,omitempty"`
	IdleTimeoutSec int    `json:"idle_timeout_sec,omitempty"` // 0 = no timeout
}

// SubjectPrefix returns the NATS subject prefix for a session.
func SubjectPrefix(sessionID string) string {
	return fmt.Sprintf("grlx.shell.%s", sessionID)
}

// Subjects returns all NATS subjects for a given session ID.
func Subjects(sessionID string) StartResponse {
	prefix := SubjectPrefix(sessionID)
	return StartResponse{
		SessionID:     sessionID,
		InputSubject:  prefix + ".input",
		OutputSubject: prefix + ".output",
		ResizeSubject: prefix + ".resize",
		DoneSubject:   prefix + ".done",
	}
}
