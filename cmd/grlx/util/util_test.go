package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func init() {
	// Disable color output for deterministic test comparisons.
	color.NoColor = true
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestOutputError_TextMode(t *testing.T) {
	testErr := errors.New("something went wrong")
	out := captureStdout(t, func() {
		OutputError(testErr, "text")
	})
	if !strings.Contains(out, "something went wrong") {
		t.Errorf("expected error message in output, got: %s", out)
	}
}

func TestOutputError_EmptyModeDefaultsToText(t *testing.T) {
	testErr := errors.New("default mode error")
	out := captureStdout(t, func() {
		OutputError(testErr, "")
	})
	if !strings.Contains(out, "default mode error") {
		t.Errorf("expected error message in output, got: %s", out)
	}
}

func TestOutputError_JSONMode(t *testing.T) {
	testErr := errors.New("json error")
	out := captureStdout(t, func() {
		OutputError(testErr, "json")
	})
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s (err: %v)", out, err)
	}
	if result["success"] != false {
		t.Errorf("expected success=false, got: %v", result["success"])
	}
}

func TestWriteJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	out := captureStdout(t, func() {
		WriteJSON(data)
	})
	trimmed := strings.TrimSpace(out)
	var result map[string]string
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", out)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got: %v", result["key"])
	}
}

func TestWriteOutput_JSON(t *testing.T) {
	data := map[string]int{"count": 42}
	out := captureStdout(t, func() {
		WriteOutput(data, "json")
	})
	trimmed := strings.TrimSpace(out)
	var result map[string]int
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", out)
	}
	if result["count"] != 42 {
		t.Errorf("expected count=42, got: %v", result["count"])
	}
}

func TestWriteOutput_Text(t *testing.T) {
	out := captureStdout(t, func() {
		WriteOutput("hello world", "text")
	})
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %s", out)
	}
}

func TestWriteOutput_EmptyMode(t *testing.T) {
	out := captureStdout(t, func() {
		WriteOutput("default text", "")
	})
	if !strings.Contains(out, "default text") {
		t.Errorf("expected 'default text' in output, got: %s", out)
	}
}
