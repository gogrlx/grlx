package cmd

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestEdgeEmptyCommand(t *testing.T) {
	c := Cmd{id: "test", method: "run", params: map[string]interface{}{"name": ""}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestEdgeWhitespaceOnlyCommand(t *testing.T) {
	c := Cmd{id: "test", method: "run", params: map[string]interface{}{"name": "   \t  "}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for whitespace-only command")
	}
}

func TestEdgeCommandWithLeadingTrailingSpaces(t *testing.T) {
	out, ok, err := runCmd(t, "  echo trimmed  ", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "trimmed" {
		t.Errorf("expected 'trimmed', got %q", strings.TrimSpace(out))
	}
}

func TestEdgeCommandWithTab(t *testing.T) {
	out, ok, err := runCmd(t, "echo\thello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("expected 'hello', got %q", strings.TrimSpace(out))
	}
}

func TestEdgeNonStringName(t *testing.T) {
	c := Cmd{id: "test", method: "run", params: map[string]interface{}{"name": 42}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for non-string name")
	}
}

func TestEdgeNilName(t *testing.T) {
	c := Cmd{id: "test", method: "run", params: map[string]interface{}{"name": nil}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for nil name")
	}
}

func TestEdgeCommandNotFound(t *testing.T) {
	_, ok, _ := runCmd(t, "nonexistent_command_xyz_123", nil)
	if ok {
		t.Error("expected failure for nonexistent command")
	}
}

func TestEdgeExitCode(t *testing.T) {
	_, ok, _ := runCmd(t, "false", nil)
	if ok {
		t.Error("expected failure for 'false' command")
	}
}

func TestEdgeExitCodeSuccess(t *testing.T) {
	_, ok, err := runCmd(t, "true", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected success for 'true' command")
	}
}

func TestEdgeTimeout(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":    "sleep 30",
			"timeout": "500ms",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, _ := c.run(ctx, false)
	if result.Succeeded {
		t.Error("expected failure due to timeout")
	}
}

func TestEdgeContextCancellation(t *testing.T) {
	c := Cmd{id: "test", method: "run", params: map[string]interface{}{"name": "sleep 30"}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result, _ := c.run(ctx, false)
	if result.Succeeded {
		t.Error("expected failure due to context cancellation")
	}
}

func TestEdgeTestMode(t *testing.T) {
	c := Cmd{id: "test", method: "run", params: map[string]interface{}{"name": "echo should_not_run"}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, note := range result.Notes {
		if strings.Contains(note.String(), "would have been run") {
			found = true
		}
		if strings.Contains(note.String(), "should_not_run") {
			t.Error("test mode should not actually run the command")
		}
	}
	if !found {
		t.Error("expected 'would have been run' note in test mode")
	}
}

func TestEdgeCwd(t *testing.T) {
	out, ok, err := runCmd(t, "pwd", map[string]interface{}{"cwd": "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "/tmp" {
		t.Errorf("expected '/tmp', got %q", strings.TrimSpace(out))
	}
}

func TestEdgeEnvVars(t *testing.T) {
	out, ok, err := runCmd(t, "env | grep GRLX_TEST_VAR", map[string]interface{}{
		"env": []string{"GRLX_TEST_VAR=hello123"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "GRLX_TEST_VAR=hello123") {
		t.Errorf("expected env var in output, got %q", out)
	}
}

func TestEdgeLongOutput(t *testing.T) {
	out, ok, err := runCmd(t, `seq 1 1000`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1000 {
		t.Errorf("expected 1000 lines, got %d", len(lines))
	}
}

func TestEdgeSpecialCharsInArgs(t *testing.T) {
	// Ensure special chars in arguments are preserved
	out, ok, err := runCmd(t, `echo "hello@world#123!yes"`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "hello@world#123!yes" {
		t.Errorf("expected 'hello@world#123!yes', got %q", strings.TrimSpace(out))
	}
}

func TestNeedsShell(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"echo hello", false},
		{"ls -la /tmp", false},
		{"whoami", false},
		{"echo hello | grep h", true},
		{"echo hello > /tmp/out", true},
		{"cat < /tmp/in", true},
		{"echo $HOME", true},
		{"echo `date`", true},
		{`echo "quoted"`, true},
		{`echo 'quoted'`, true},
		{"echo one; echo two", true},
		{"true && echo yes", true},
		{"false || echo no", true},
		{"sleep 1 &", true},
		{`echo hello\nworld`, true},
		{"echo one\necho two", true},
	}
	for _, tt := range tests {
		got := needsShell(tt.cmd)
		if got != tt.want {
			t.Errorf("needsShell(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}
