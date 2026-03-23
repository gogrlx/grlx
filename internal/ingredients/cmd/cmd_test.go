package cmd

import (
	"context"
	"testing"
	"time"
)

func TestParse_ValidRunMethod(t *testing.T) {
	c := Cmd{}
	cooker, err := c.Parse("step1", "run", map[string]interface{}{
		"name": "echo hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cooker == nil {
		t.Fatal("expected non-nil cooker")
	}
}

func TestParse_NilParams(t *testing.T) {
	c := Cmd{}
	// nil params should default to empty map — "name" is required, so should fail
	_, err := c.Parse("step1", "run", nil)
	if err == nil {
		t.Fatal("expected error for nil params (missing required 'name')")
	}
}

func TestParse_MissingName(t *testing.T) {
	c := Cmd{}
	_, err := c.Parse("step1", "run", map[string]interface{}{
		"cwd": "/tmp",
	})
	if err == nil {
		t.Fatal("expected error for missing 'name'")
	}
}

func TestParse_EmptyName(t *testing.T) {
	c := Cmd{}
	_, err := c.Parse("step1", "run", map[string]interface{}{
		"name": "",
	})
	if err == nil {
		t.Fatal("expected error for empty 'name'")
	}
}

func TestParse_NonStringName(t *testing.T) {
	c := Cmd{}
	_, err := c.Parse("step1", "run", map[string]interface{}{
		"name": 42,
	})
	if err == nil {
		t.Fatal("expected error for non-string 'name'")
	}
}

func TestParse_UndefinedMethod(t *testing.T) {
	c := Cmd{}
	_, err := c.Parse("step1", "nonexistent", map[string]interface{}{
		"name": "echo hello",
	})
	if err == nil {
		t.Fatal("expected error for undefined method")
	}
}

func TestParse_WithOptionalParams(t *testing.T) {
	c := Cmd{}
	cooker, err := c.Parse("step1", "run", map[string]interface{}{
		"name":    "echo hello",
		"cwd":     "/tmp",
		"timeout": "5s",
		"runas":   "nobody",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cooker == nil {
		t.Fatal("expected non-nil cooker")
	}
}

func TestPropertiesForMethod_Run(t *testing.T) {
	c := Cmd{}
	props, err := c.PropertiesForMethod("run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if props == nil {
		t.Fatal("expected non-nil props")
	}
	// Verify known keys exist
	expectedKeys := []string{"name", "args", "env", "cwd", "runas", "path", "timeout"}
	for _, key := range expectedKeys {
		if _, ok := props[key]; !ok {
			t.Errorf("missing expected property key %q", key)
		}
	}
}

func TestPropertiesForMethod_Undefined(t *testing.T) {
	c := Cmd{}
	_, err := c.PropertiesForMethod("nonexistent")
	if err == nil {
		t.Fatal("expected error for undefined method")
	}
}

func TestMethods(t *testing.T) {
	c := Cmd{}
	ingredient, methods := c.Methods()
	if ingredient != "cmd" {
		t.Errorf("expected ingredient 'cmd', got %q", ingredient)
	}
	if len(methods) != 1 || methods[0] != "run" {
		t.Errorf("expected methods [run], got %v", methods)
	}
}

func TestProperties(t *testing.T) {
	c := Cmd{
		id:     "step1",
		method: "run",
		params: map[string]interface{}{
			"name":    "echo hello",
			"cwd":     "/tmp",
			"timeout": "5s",
		},
	}
	props, err := c.Properties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if props["name"] != "echo hello" {
		t.Errorf("expected name 'echo hello', got %v", props["name"])
	}
	if props["cwd"] != "/tmp" {
		t.Errorf("expected cwd '/tmp', got %v", props["cwd"])
	}
}

func TestProperties_Empty(t *testing.T) {
	c := Cmd{params: map[string]interface{}{}}
	props, err := c.Properties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(props) != 0 {
		t.Errorf("expected empty props, got %v", props)
	}
}

func TestTest_Run(t *testing.T) {
	c := Cmd{
		id:     "step1",
		method: "run",
		params: map[string]interface{}{
			"name": "echo test_mode",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.Test(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Test mode should not execute the command
	found := false
	for _, note := range result.Notes {
		if note.String() == "Command would have been run" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'would have been run' note in test mode")
	}
}

func TestTest_UndefinedMethod(t *testing.T) {
	c := Cmd{
		id:     "step1",
		method: "nonexistent",
		params: map[string]interface{}{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.Test(ctx)
	if err == nil {
		t.Fatal("expected error for undefined method")
	}
	if !result.Failed {
		t.Error("expected Failed=true for undefined method")
	}
}

func TestApply_Run(t *testing.T) {
	c := Cmd{
		id:     "step1",
		method: "run",
		params: map[string]interface{}{
			"name": "echo apply_test",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.Apply(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Succeeded {
		t.Error("expected success")
	}
}

func TestApply_UndefinedMethod(t *testing.T) {
	c := Cmd{
		id:     "step1",
		method: "nonexistent",
		params: map[string]interface{}{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.Apply(ctx)
	if err == nil {
		t.Fatal("expected error for undefined method")
	}
	if !result.Failed {
		t.Error("expected Failed=true for undefined method")
	}
}
