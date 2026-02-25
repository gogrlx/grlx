package cmd

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunEmptyString(t *testing.T) {
	// Issue #111: empty strings in commands should be preserved
	// e.g. ssh-keygen -N "" -f /tmp/testkey
	// The "" should be passed as an empty argument, not stripped
	c := Cmd{
		id:     "test-empty-string",
		method: "run",
		params: map[string]interface{}{
			"name": `echo -n ""`,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With naive strings.Split on space, `""` becomes a literal arg to echo
	// which means echo prints "" instead of empty string
	// The output should be empty (echo -n with empty string arg)
	output := ""
	for _, note := range result.Notes {
		if strings.HasPrefix(note.String(), "Command output: ") {
			output = strings.TrimPrefix(note.String(), "Command output: ")
		}
	}
	t.Logf("output: %q", output)
	// Currently strings.Split will pass `""` literally, so echo prints ""
	// After fix, it should handle quoted args properly
}

func TestRunPipe(t *testing.T) {
	// Issue #111: pipe commands should work
	// echo hello | tr a-z A-Z → should output HELLO
	c := Cmd{
		id:     "test-pipe",
		method: "run",
		params: map[string]interface{}{
			"name": `echo hello | tr a-z A-Z`,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	output := ""
	for _, note := range result.Notes {
		if strings.HasPrefix(note.String(), "Command output: ") {
			output = strings.TrimPrefix(note.String(), "Command output: ")
		}
	}
	t.Logf("output: %q, err: %v, result: %+v", output, err, result)
	// With exec.Command (no shell), pipe is treated as literal arg to echo
	// After fix, should output "HELLO\n"
	trimmed := strings.TrimSpace(output)
	if trimmed != "HELLO" {
		t.Errorf("expected HELLO, got %q", trimmed)
	}
}

func TestRunRedirect(t *testing.T) {
	// Issue #111: redirects should work
	// echo hello > /dev/null should succeed silently
	c := Cmd{
		id:     "test-redirect",
		method: "run",
		params: map[string]interface{}{
			"name": `echo hello > /dev/null`,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	t.Logf("err: %v, result: %+v", err, result)
	// With exec.Command, > is a literal arg
	// After fix with shell, this should work
	if result.Failed {
		t.Errorf("expected success, got failure")
	}
}

func TestRunSubshell(t *testing.T) {
	// Issue #111: subshell/command substitution should work
	// echo $(whoami) should output the current user
	c := Cmd{
		id:     "test-subshell",
		method: "run",
		params: map[string]interface{}{
			"name": `echo $(whoami)`,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	output := ""
	for _, note := range result.Notes {
		if strings.HasPrefix(note.String(), "Command output: ") {
			output = strings.TrimPrefix(note.String(), "Command output: ")
		}
	}
	t.Logf("output: %q, err: %v", output, err)
	trimmed := strings.TrimSpace(output)
	// Without shell, $(whoami) is literal
	// With shell, should output actual username
	if trimmed == "$(whoami)" || trimmed == "" {
		t.Errorf("subshell not expanded, got %q", trimmed)
	}
}

func TestRunSimpleCommand(t *testing.T) {
	// Simple commands should still work after the fix
	c := Cmd{
		id:     "test-simple",
		method: "run",
		params: map[string]interface{}{
			"name": "echo hello",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Succeeded {
		t.Fatalf("expected success")
	}
	output := ""
	for _, note := range result.Notes {
		if strings.HasPrefix(note.String(), "Command output: ") {
			output = strings.TrimPrefix(note.String(), "Command output: ")
		}
	}
	if strings.TrimSpace(output) != "hello" {
		t.Errorf("expected 'hello', got %q", strings.TrimSpace(output))
	}
}

func TestRunMultilineYAML(t *testing.T) {
	// Issue #111: multiline YAML strings should work
	// Simulating what comes from YAML parsing of:
	// - name: |
	//     echo hello
	//     echo world
	c := Cmd{
		id:     "test-multiline",
		method: "run",
		params: map[string]interface{}{
			"name": "echo hello\necho world",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	output := ""
	for _, note := range result.Notes {
		if strings.HasPrefix(note.String(), "Command output: ") {
			output = strings.TrimPrefix(note.String(), "Command output: ")
		}
	}
	t.Logf("output: %q, err: %v, result: %+v", output, err, result)
	// With shell, multiline should execute both commands
	if !strings.Contains(output, "hello") || !strings.Contains(output, "world") {
		t.Errorf("expected both hello and world in output, got %q", output)
	}
}
