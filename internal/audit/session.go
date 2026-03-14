package audit

import (
	"encoding/json"
	"time"
)

// SessionEntry represents an audit log record for an interactive shell session.
// Session events use the same JSONL log files as command audit entries.
type SessionEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Pubkey    string    `json:"pubkey"`
	RoleName  string    `json:"role"`
	Action    string    `json:"action"` // "session.start" or "session.end"
	SessionID string    `json:"session_id"`
	SproutID  string    `json:"sprout_id"`
	Shell     string    `json:"shell,omitempty"`
	Duration  float64   `json:"duration_secs,omitempty"` // seconds, only on session.end
	BytesIn   int64     `json:"bytes_in,omitempty"`      // CLI → sprout, only on session.end
	BytesOut  int64     `json:"bytes_out,omitempty"`     // sprout → CLI, only on session.end
	ExitCode  *int      `json:"exit_code,omitempty"`     // only on session.end
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// LogSessionStart records the beginning of an SSH shell session.
func LogSessionStart(pubkey, roleName, sessionID, sproutID, shell string) error {
	l := Global()
	if l == nil {
		return nil
	}

	data, err := json.Marshal(SessionEntry{
		Timestamp: time.Now().UTC(),
		Pubkey:    pubkey,
		RoleName:  roleName,
		Action:    "session.start",
		SessionID: sessionID,
		SproutID:  sproutID,
		Shell:     shell,
		Success:   true,
	})
	if err != nil {
		return err
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureFile(); err != nil {
		return err
	}
	_, err = l.file.Write(data)
	return err
}

// SessionEndInfo contains the details needed to log a session.end event.
type SessionEndInfo struct {
	Pubkey    string
	RoleName  string
	SessionID string
	SproutID  string
	Shell     string
	StartTime time.Time
	BytesIn   int64
	BytesOut  int64
	ExitCode  int
	Error     string
}

// LogSessionEnd records the end of an SSH shell session.
func LogSessionEnd(info SessionEndInfo) error {
	l := Global()
	if l == nil {
		return nil
	}

	now := time.Now().UTC()
	duration := now.Sub(info.StartTime).Seconds()

	exitCode := info.ExitCode
	entry := SessionEntry{
		Timestamp: now,
		Pubkey:    info.Pubkey,
		RoleName:  info.RoleName,
		Action:    "session.end",
		SessionID: info.SessionID,
		SproutID:  info.SproutID,
		Shell:     info.Shell,
		Duration:  duration,
		BytesIn:   info.BytesIn,
		BytesOut:  info.BytesOut,
		ExitCode:  &exitCode,
		Success:   info.Error == "",
		Error:     info.Error,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureFile(); err != nil {
		return err
	}
	_, err = l.file.Write(data)
	return err
}
