package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogSessionStart(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	SetGlobal(logger)
	defer SetGlobal(nil)

	err = LogSessionStart("PUBKEY_ABC", "admin", "sess-001", "web-1", "/bin/bash")
	if err != nil {
		t.Fatalf("LogSessionStart: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got SessionEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Action != "session.start" {
		t.Errorf("action = %q, want session.start", got.Action)
	}
	if got.Pubkey != "PUBKEY_ABC" {
		t.Errorf("pubkey = %q, want PUBKEY_ABC", got.Pubkey)
	}
	if got.RoleName != "admin" {
		t.Errorf("role = %q, want admin", got.RoleName)
	}
	if got.SessionID != "sess-001" {
		t.Errorf("session_id = %q, want sess-001", got.SessionID)
	}
	if got.SproutID != "web-1" {
		t.Errorf("sprout_id = %q, want web-1", got.SproutID)
	}
	if got.Shell != "/bin/bash" {
		t.Errorf("shell = %q, want /bin/bash", got.Shell)
	}
	if !got.Success {
		t.Error("success = false, want true")
	}
	if got.Timestamp.IsZero() {
		t.Error("timestamp is zero")
	}
}

func TestLogSessionEnd(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	SetGlobal(logger)
	defer SetGlobal(nil)

	startTime := time.Now().UTC().Add(-5 * time.Minute)
	info := SessionEndInfo{
		Pubkey:    "PUBKEY_XYZ",
		RoleName:  "operator",
		SessionID: "sess-002",
		SproutID:  "db-1",
		Shell:     "/bin/sh",
		StartTime: startTime,
		BytesIn:   1024,
		BytesOut:  8192,
		ExitCode:  0,
	}

	err = LogSessionEnd(info)
	if err != nil {
		t.Fatalf("LogSessionEnd: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got SessionEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Action != "session.end" {
		t.Errorf("action = %q, want session.end", got.Action)
	}
	if got.Pubkey != "PUBKEY_XYZ" {
		t.Errorf("pubkey = %q, want PUBKEY_XYZ", got.Pubkey)
	}
	if got.RoleName != "operator" {
		t.Errorf("role = %q, want operator", got.RoleName)
	}
	if got.SessionID != "sess-002" {
		t.Errorf("session_id = %q, want sess-002", got.SessionID)
	}
	if got.SproutID != "db-1" {
		t.Errorf("sprout_id = %q, want db-1", got.SproutID)
	}
	if got.BytesIn != 1024 {
		t.Errorf("bytes_in = %d, want 1024", got.BytesIn)
	}
	if got.BytesOut != 8192 {
		t.Errorf("bytes_out = %d, want 8192", got.BytesOut)
	}
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Errorf("exit_code = %v, want 0", got.ExitCode)
	}
	if got.Duration < 299 || got.Duration > 301 {
		t.Errorf("duration = %.1f, want ~300", got.Duration)
	}
	if !got.Success {
		t.Error("success = false, want true")
	}
}

func TestLogSessionEndWithError(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	SetGlobal(logger)
	defer SetGlobal(nil)

	info := SessionEndInfo{
		Pubkey:    "PUBKEY_ERR",
		RoleName:  "dev",
		SessionID: "sess-003",
		SproutID:  "app-1",
		Shell:     "/bin/zsh",
		StartTime: time.Now().UTC().Add(-10 * time.Second),
		BytesIn:   256,
		BytesOut:  512,
		ExitCode:  1,
		Error:     "signal: killed",
	}

	err = LogSessionEnd(info)
	if err != nil {
		t.Fatalf("LogSessionEnd: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got SessionEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Success {
		t.Error("success = true, want false")
	}
	if got.Error != "signal: killed" {
		t.Errorf("error = %q, want 'signal: killed'", got.Error)
	}
	if got.ExitCode == nil || *got.ExitCode != 1 {
		t.Errorf("exit_code = %v, want 1", got.ExitCode)
	}
}

func TestLogSessionStartAndEnd(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	SetGlobal(logger)
	defer SetGlobal(nil)

	// Log a start and end in sequence.
	err = LogSessionStart("PK1", "admin", "sess-004", "worker-1", "/bin/bash")
	if err != nil {
		t.Fatalf("LogSessionStart: %v", err)
	}

	err = LogSessionEnd(SessionEndInfo{
		Pubkey:    "PK1",
		RoleName:  "admin",
		SessionID: "sess-004",
		SproutID:  "worker-1",
		Shell:     "/bin/bash",
		StartTime: time.Now().UTC().Add(-2 * time.Minute),
		BytesIn:   500,
		BytesOut:  3000,
		ExitCode:  0,
	})
	if err != nil {
		t.Fatalf("LogSessionEnd: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var start SessionEntry
	if err := json.Unmarshal([]byte(lines[0]), &start); err != nil {
		t.Fatalf("Unmarshal start: %v", err)
	}
	if start.Action != "session.start" {
		t.Errorf("first entry action = %q, want session.start", start.Action)
	}

	var end SessionEntry
	if err := json.Unmarshal([]byte(lines[1]), &end); err != nil {
		t.Fatalf("Unmarshal end: %v", err)
	}
	if end.Action != "session.end" {
		t.Errorf("second entry action = %q, want session.end", end.Action)
	}

	// Both should reference the same session.
	if start.SessionID != end.SessionID {
		t.Errorf("session IDs differ: start=%q end=%q", start.SessionID, end.SessionID)
	}
}

func TestLogSessionNoGlobalLogger(t *testing.T) {
	// Ensure no global logger is set.
	SetGlobal(nil)

	// Should return nil (no-op) when no logger is configured.
	err := LogSessionStart("PK", "role", "sess", "sprout", "/bin/sh")
	if err != nil {
		t.Errorf("LogSessionStart without global logger: %v", err)
	}

	err = LogSessionEnd(SessionEndInfo{
		SessionID: "sess",
		StartTime: time.Now().UTC(),
	})
	if err != nil {
		t.Errorf("LogSessionEnd without global logger: %v", err)
	}
}
