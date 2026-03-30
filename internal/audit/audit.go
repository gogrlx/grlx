// Package audit provides per-user audit logging for grlx farmer actions.
//
// Every authenticated API action (cook, key accept, job cancel, prop changes,
// etc.) is recorded as a JSONL entry with the user's identity, timestamp,
// action type, and relevant targets.
//
// The audit log is append-only and written to a configurable directory
// (default: /var/log/grlx/audit/). Files are rotated daily.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single audit log record.
type Entry struct {
	Timestamp  time.Time       `json:"timestamp"`
	Username   string          `json:"username,omitempty"`
	Pubkey     string          `json:"pubkey"`
	RoleName   string          `json:"role"`
	Action     string          `json:"action"`
	Targets    []string        `json:"targets,omitempty"`
	Parameters json.RawMessage `json:"params,omitempty"`
	Success    bool            `json:"success"`
	Error      string          `json:"error,omitempty"`
}

// Logger writes audit entries to JSONL files.
type Logger struct {
	mu      sync.Mutex
	dir     string
	file    *os.File
	current string // current file date (YYYY-MM-DD)
}

// NewLogger creates an audit logger that writes to the given directory.
// The directory is created if it does not exist.
func NewLogger(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("audit: create dir %s: %w", dir, err)
	}
	return &Logger{dir: dir}, nil
}

// Log writes an audit entry to the current day's log file.
func (l *Logger) Log(entry Entry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("audit: marshal entry: %w", err)
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

// Close closes the underlying file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		l.current = ""
		return err
	}
	return nil
}

// ensureFile opens or rotates the log file based on the current date.
// Must be called with l.mu held.
func (l *Logger) ensureFile() error {
	today := time.Now().UTC().Format("2006-01-02")
	if l.file != nil && l.current == today {
		return nil
	}

	if l.file != nil {
		l.file.Close()
		l.file = nil
	}

	path := filepath.Join(l.dir, fmt.Sprintf("audit-%s.jsonl", today))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("audit: open %s: %w", path, err)
	}

	l.file = f
	l.current = today
	return nil
}

// Dir returns the configured audit log directory.
func (l *Logger) Dir() string {
	return l.dir
}
