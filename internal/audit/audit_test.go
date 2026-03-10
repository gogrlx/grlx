package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "nested", "audit")

	logger, err := NewLogger(subdir)
	if err != nil {
		t.Fatalf("NewLogger(%q): %v", subdir, err)
	}
	defer logger.Close()

	info, err := os.Stat(subdir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestLogEntry(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	entry := Entry{
		Timestamp: time.Date(2026, 3, 10, 5, 30, 0, 0, time.UTC),
		Pubkey:    "APUBKEY123",
		RoleName:  "admin",
		Action:    "cook",
		Targets:   []string{"web-1", "web-2"},
		Success:   true,
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Read the file
	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var got Entry
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Pubkey != "APUBKEY123" {
		t.Errorf("pubkey = %q, want APUBKEY123", got.Pubkey)
	}
	if got.Action != "cook" {
		t.Errorf("action = %q, want cook", got.Action)
	}
	if len(got.Targets) != 2 {
		t.Errorf("targets = %v, want [web-1 web-2]", got.Targets)
	}
	if !got.Success {
		t.Error("success = false, want true")
	}
}

func TestLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	for i := 0; i < 5; i++ {
		err := logger.Log(Entry{
			Pubkey:   "APUBKEY",
			RoleName: "dev",
			Action:   "props.set",
			Success:  true,
		})
		if err != nil {
			t.Fatalf("Log entry %d: %v", i, err)
		}
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
}

func TestLogWithError(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	entry := Entry{
		Pubkey:   "APUBKEY456",
		RoleName: "readonly",
		Action:   "pki.accept",
		Targets:  []string{"db-1"},
		Success:  false,
		Error:    "access denied",
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Error != "access denied" {
		t.Errorf("error = %q, want 'access denied'", got.Error)
	}
	if got.Success {
		t.Error("success = true, want false")
	}
}

func TestLogWithParams(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	params, _ := json.Marshal(map[string]string{"recipe": "nginx.sls"})
	entry := Entry{
		Pubkey:     "APUBKEY789",
		RoleName:   "sre",
		Action:     "cook",
		Parameters: params,
		Success:    true,
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	var gotParams map[string]string
	if err := json.Unmarshal(got.Parameters, &gotParams); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}
	if gotParams["recipe"] != "nginx.sls" {
		t.Errorf("params.recipe = %q, want nginx.sls", gotParams["recipe"])
	}
}

func TestLogDefaultTimestamp(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	before := time.Now().UTC()
	err = logger.Log(Entry{
		Pubkey:   "APUBKEY",
		RoleName: "admin",
		Action:   "version",
		Success:  true,
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	after := time.Now().UTC()

	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Timestamp.Before(before) || got.Timestamp.After(after) {
		t.Errorf("timestamp %v not between %v and %v", got.Timestamp, before, after)
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	// Log something to open a file
	err = logger.Log(Entry{Action: "test", Success: true})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Closing again should be safe
	if err := logger.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// Logging after close should reopen
	err = logger.Log(Entry{Action: "test2", Success: true})
	if err != nil {
		t.Fatalf("Log after Close: %v", err)
	}
	logger.Close()
}
