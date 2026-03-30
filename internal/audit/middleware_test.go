package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsReadOnly(t *testing.T) {
	readOnly := []string{"version", "sprouts.list", "jobs.list", "auth.whoami"}
	for _, a := range readOnly {
		if !IsReadOnly(a) {
			t.Errorf("IsReadOnly(%q) = false, want true", a)
		}
	}

	writeActions := []string{"cook", "pki.accept", "props.set", "jobs.cancel"}
	for _, a := range writeActions {
		if IsReadOnly(a) {
			t.Errorf("IsReadOnly(%q) = true, want false", a)
		}
	}
}

func TestRedactToken(t *testing.T) {
	input := `{"token":"secret123","sprout_id":"web-1"}`
	result := redactToken(json.RawMessage(input))

	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, hasToken := m["token"]; hasToken {
		t.Error("token field should be redacted")
	}
	if _, hasSprout := m["sprout_id"]; !hasSprout {
		t.Error("sprout_id field should be preserved")
	}
}

func TestRedactTokenEmpty(t *testing.T) {
	// No token field — should return unchanged
	input := `{"sprout_id":"web-1"}`
	result := redactToken(json.RawMessage(input))

	if string(result) != input {
		t.Errorf("expected unchanged input, got %s", string(result))
	}
}

func TestRedactTokenOnlyToken(t *testing.T) {
	input := `{"token":"secret123"}`
	result := redactToken(json.RawMessage(input))

	if result != nil {
		t.Errorf("expected nil for token-only params, got %s", string(result))
	}
}

func TestExtractTargets(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   []string
	}{
		{
			name:   "sprout_id",
			params: `{"sprout_id":"web-1"}`,
			want:   []string{"web-1"},
		},
		{
			name:   "sprout_ids",
			params: `{"sprout_ids":["web-1","web-2"]}`,
			want:   []string{"web-1", "web-2"},
		},
		{
			name:   "target array",
			params: `{"target":[{"SproutID":"db-1"},{"SproutID":"db-2"}]}`,
			want:   []string{"db-1", "db-2"},
		},
		{
			name:   "empty",
			params: `{}`,
			want:   nil,
		},
		{
			name:   "nil",
			params: "",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params json.RawMessage
			if tt.params != "" {
				params = json.RawMessage(tt.params)
			}
			got := extractTargets(params)
			if len(got) != len(tt.want) {
				t.Errorf("extractTargets() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractTargets()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLogActionWithGlobal(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	SetGlobal(logger)
	defer SetGlobal(nil)

	// Set a mock identity resolver
	SetIdentityResolver(func(token string) (string, string, string, error) {
		if token == "test-token" {
			return "APUBKEY_TEST", "admin", "alice", nil
		}
		return "", "", "", nil
	})
	defer SetIdentityResolver(nil)

	params := json.RawMessage(`{"token":"test-token","sprout_id":"web-1"}`)
	err = LogAction("pki.accept", params, nil, nil)
	if err != nil {
		t.Fatalf("LogAction: %v", err)
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

	if got.Pubkey != "APUBKEY_TEST" {
		t.Errorf("pubkey = %q, want APUBKEY_TEST", got.Pubkey)
	}
	if got.RoleName != "admin" {
		t.Errorf("role = %q, want admin", got.RoleName)
	}
	if got.Username != "alice" {
		t.Errorf("username = %q, want alice", got.Username)
	}
	if got.Action != "pki.accept" {
		t.Errorf("action = %q, want pki.accept", got.Action)
	}
	if !got.Success {
		t.Error("success = false, want true")
	}
	if len(got.Targets) != 1 || got.Targets[0] != "web-1" {
		t.Errorf("targets = %v, want [web-1]", got.Targets)
	}

	// Verify token is redacted from params
	if got.Parameters != nil {
		var pm map[string]json.RawMessage
		if err := json.Unmarshal(got.Parameters, &pm); err == nil {
			if _, hasToken := pm["token"]; hasToken {
				t.Error("token should be redacted from params")
			}
		}
	}
}

func TestLogActionNoGlobal(t *testing.T) {
	SetGlobal(nil)

	// Should silently succeed
	err := LogAction("cook", nil, nil, nil)
	if err != nil {
		t.Fatalf("LogAction with no global: %v", err)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"all", LevelAll},
		{"write", LevelWrite},
		{"off", LevelOff},
		{"", LevelWrite},
		{"invalid", LevelWrite},
		{"ALL", LevelWrite}, // case-sensitive
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShouldLog(t *testing.T) {
	tests := []struct {
		level  Level
		action string
		want   bool
	}{
		{LevelAll, "cook", true},
		{LevelAll, "version", true},
		{LevelAll, "sprouts.list", true},
		{LevelWrite, "cook", true},
		{LevelWrite, "pki.accept", true},
		{LevelWrite, "version", false},
		{LevelWrite, "sprouts.list", false},
		{LevelOff, "cook", false},
		{LevelOff, "version", false},
	}

	for _, tt := range tests {
		SetLevel(tt.level)
		got := ShouldLog(tt.action)
		if got != tt.want {
			t.Errorf("ShouldLog(%q) at level %q = %v, want %v",
				tt.action, tt.level, got, tt.want)
		}
	}

	// Restore default
	SetLevel(LevelWrite)
}

func TestSetAndGetLevel(t *testing.T) {
	SetLevel(LevelAll)
	if got := GetLevel(); got != LevelAll {
		t.Errorf("GetLevel() = %q, want %q", got, LevelAll)
	}
	SetLevel(LevelOff)
	if got := GetLevel(); got != LevelOff {
		t.Errorf("GetLevel() = %q, want %q", got, LevelOff)
	}
	// Restore
	SetLevel(LevelWrite)
}

func TestLogActionReadOnlySkipped(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	SetGlobal(logger)
	defer SetGlobal(nil)

	// Read-only actions should still work via LogAction (the caller
	// in router.go skips them, but LogAction itself doesn't reject them)
	err = LogAction("version", nil, nil, nil)
	if err != nil {
		t.Fatalf("LogAction: %v", err)
	}
}
