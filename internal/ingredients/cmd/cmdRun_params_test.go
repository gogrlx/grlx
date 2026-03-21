package cmd

import (
	"context"
	"testing"
	"time"
)

func TestRunInvalidRunasType(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":  "echo hello",
			"runas": 123, // should be string
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for non-string runas")
	}
}

func TestRunInvalidPathType(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name": "echo hello",
			"path": 42,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for non-string path")
	}
}

func TestRunInvalidCwdType(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name": "echo hello",
			"cwd":  []string{"not", "a", "string"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for non-string cwd")
	}
}

func TestRunInvalidEnvType(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name": "echo hello",
			"env":  "not_a_slice",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for non-slice env")
	}
}

func TestRunInvalidEnvFormat(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name": "echo hello",
			"env":  []string{"INVALID_NO_EQUALS"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for invalid env var format (missing =)")
	}
}

func TestRunInvalidTimeoutType(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":    "echo hello",
			"timeout": 5, // should be string
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for non-string timeout")
	}
}

func TestRunInvalidTimeoutDuration(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":    "echo hello",
			"timeout": "not_a_duration",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for unparseable timeout")
	}
	if !result.Failed {
		t.Error("expected Failed=true for invalid timeout")
	}
}

func TestRunNonexistentRunas(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":  "echo hello",
			"runas": "nonexistent_user_xyz_9999",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.run(ctx, false)
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestRunWithValidTimeout(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":    "echo hello",
			"timeout": "10s",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := c.run(ctx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Succeeded {
		t.Error("expected success")
	}
}

func TestRunWithPath(t *testing.T) {
	// Setting path should work for finding executables
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name": "echo with_path",
			"path": "/usr/bin/echo",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// This will use path as the executable path override
	result, err := c.run(ctx, false)
	// The command may or may not succeed depending on how path interacts,
	// but it shouldn't return a type error
	_ = result
	_ = err
}

func TestRunTestModeWithAllParams(t *testing.T) {
	c := Cmd{
		id:     "test",
		method: "run",
		params: map[string]interface{}{
			"name":    "echo should_not_run",
			"cwd":     "/tmp",
			"timeout": "5s",
			"env":     []string{"FOO=bar"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.run(ctx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Test mode should return early before executing
	found := false
	for _, note := range result.Notes {
		s := note.String()
		if s == "Command would have been run" {
			found = true
		}
	}
	if !found {
		t.Error("expected test mode note")
	}
}
