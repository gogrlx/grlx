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

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
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

// withStdin replaces os.Stdin with a pipe containing the given input,
// runs fn, then restores the original stdin.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_, err = w.WriteString(input)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	defer func() { os.Stdin = origStdin }()
	fn()
}

// --- OutputError tests ---

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

// --- WriteJSON / WriteJSONErr / WriteOutput tests ---

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

func TestWriteJSONErr(t *testing.T) {
	testErr := errors.New("test write json err")
	out := captureStdout(t, func() {
		WriteJSONErr(testErr)
	})
	trimmed := strings.TrimSpace(out)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", out)
	}
	if result["success"] != false {
		t.Errorf("expected success=false, got: %v", result["success"])
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

// --- IsInteractive tests ---

func TestIsInteractive(t *testing.T) {
	// In tests, stdin is typically a pipe, not a terminal.
	if IsInteractive() {
		t.Error("expected IsInteractive()=false in test environment")
	}
}

// --- UserChoice tests ---

func TestUserChoice_SelectsFirst(t *testing.T) {
	withStdin(t, "yes\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoice("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "yes" {
				t.Errorf("expected 'yes', got %q", result)
			}
		})
	})
}

func TestUserChoice_SelectsSecond(t *testing.T) {
	withStdin(t, "no\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoice("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "no" {
				t.Errorf("expected 'no', got %q", result)
			}
		})
	})
}

func TestUserChoice_CaseInsensitive(t *testing.T) {
	withStdin(t, "YES\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoice("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "yes" {
				t.Errorf("expected 'yes', got %q", result)
			}
		})
	})
}

func TestUserChoice_InvalidInput(t *testing.T) {
	withStdin(t, "maybe\n", func() {
		_ = captureStdout(t, func() {
			_, err := UserChoice("yes", "no")
			if !errors.Is(err, apitypes.ErrInvalidUserInput) {
				t.Errorf("expected ErrInvalidUserInput, got: %v", err)
			}
		})
	})
}

func TestUserChoice_WithExtraOptions(t *testing.T) {
	withStdin(t, "cancel\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoice("yes", "no", "cancel")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "cancel" {
				t.Errorf("expected 'cancel', got %q", result)
			}
		})
	})
}

func TestUserChoice_ExtraOptionCaseInsensitive(t *testing.T) {
	withStdin(t, "CANCEL\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoice("yes", "no", "cancel")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "cancel" {
				t.Errorf("expected 'cancel', got %q", result)
			}
		})
	})
}

func TestUserChoice_PanicsOnEmptyFirst(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty first option")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserChoice("", "no")
}

func TestUserChoice_PanicsOnEmptySecond(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty second option")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserChoice("yes", "")
}

func TestUserChoice_PanicsOnEmptyExtraOption(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty extra option")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserChoice("yes", "no", "")
}

// --- UserChoiceWithDefault tests ---

func TestUserChoiceWithDefault_EmptyInputReturnsDefault(t *testing.T) {
	withStdin(t, "\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoiceWithDefault("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "yes" {
				t.Errorf("expected default 'yes', got %q", result)
			}
		})
	})
}

func TestUserChoiceWithDefault_ExplicitDefault(t *testing.T) {
	withStdin(t, "yes\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoiceWithDefault("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "yes" {
				t.Errorf("expected 'yes', got %q", result)
			}
		})
	})
}

func TestUserChoiceWithDefault_SelectSecond(t *testing.T) {
	withStdin(t, "no\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoiceWithDefault("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "no" {
				t.Errorf("expected 'no', got %q", result)
			}
		})
	})
}

func TestUserChoiceWithDefault_SelectExtraOption(t *testing.T) {
	withStdin(t, "skip\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserChoiceWithDefault("yes", "no", "skip")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != "skip" {
				t.Errorf("expected 'skip', got %q", result)
			}
		})
	})
}

func TestUserChoiceWithDefault_InvalidInput(t *testing.T) {
	withStdin(t, "maybe\n", func() {
		_ = captureStdout(t, func() {
			_, err := UserChoiceWithDefault("yes", "no")
			if !errors.Is(err, apitypes.ErrInvalidUserInput) {
				t.Errorf("expected ErrInvalidUserInput, got: %v", err)
			}
		})
	})
}

func TestUserChoiceWithDefault_PanicsOnEmptyDefault(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty default")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserChoiceWithDefault("", "no")
}

func TestUserChoiceWithDefault_PanicsOnEmptySecond(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty second")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserChoiceWithDefault("yes", "")
}

func TestUserChoiceWithDefault_PanicsOnEmptyExtraOption(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty extra option")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserChoiceWithDefault("yes", "no", "")
}

// --- UserConfirm tests ---

func TestUserConfirm_First(t *testing.T) {
	withStdin(t, "yes\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirm("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result {
				t.Error("expected true for first option")
			}
		})
	})
}

func TestUserConfirm_Second(t *testing.T) {
	withStdin(t, "no\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirm("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result {
				t.Error("expected true for second option")
			}
		})
	})
}

func TestUserConfirm_InvalidReturnsFalse(t *testing.T) {
	withStdin(t, "maybe\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirm("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result {
				t.Error("expected false for invalid input")
			}
		})
	})
}

func TestUserConfirm_CaseInsensitive(t *testing.T) {
	withStdin(t, "YES\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirm("yes", "no")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result {
				t.Error("expected true for case-insensitive match")
			}
		})
	})
}

func TestUserConfirm_PanicsOnEmptyFirst(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty first option")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserConfirm("", "no")
}

func TestUserConfirm_PanicsOnEmptySecond(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on empty second option")
		}
		if r != pki.ErrConfirmationLengthIsZero {
			t.Errorf("expected ErrConfirmationLengthIsZero, got: %v", r)
		}
	}()
	_, _ = UserConfirm("yes", "")
}

// --- UserConfirmWithDefault tests ---

func TestUserConfirmWithDefault_EmptyInputDefaultTrue(t *testing.T) {
	withStdin(t, "\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirmWithDefault(true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result {
				t.Error("expected true (default)")
			}
		})
	})
}

func TestUserConfirmWithDefault_EmptyInputDefaultFalse(t *testing.T) {
	withStdin(t, "\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirmWithDefault(false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result {
				t.Error("expected false (default)")
			}
		})
	})
}

func TestUserConfirmWithDefault_ExplicitY(t *testing.T) {
	withStdin(t, "y\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirmWithDefault(false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result {
				t.Error("expected true for explicit 'y'")
			}
		})
	})
}

func TestUserConfirmWithDefault_ExplicitN(t *testing.T) {
	withStdin(t, "n\n", func() {
		_ = captureStdout(t, func() {
			result, err := UserConfirmWithDefault(true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result {
				t.Error("expected false for explicit 'n'")
			}
		})
	})
}

func TestUserConfirmWithDefault_InvalidInput(t *testing.T) {
	withStdin(t, "banana\n", func() {
		_ = captureStdout(t, func() {
			_, err := UserConfirmWithDefault(true)
			if !errors.Is(err, apitypes.ErrInvalidUserInput) {
				t.Errorf("expected ErrInvalidUserInput, got: %v", err)
			}
		})
	})
}

func TestUserConfirmWithDefault_PromptFormatTrue(t *testing.T) {
	withStdin(t, "\n", func() {
		out := captureStdout(t, func() {
			_, _ = UserConfirmWithDefault(true)
		})
		if !strings.Contains(out, "Y/n:") {
			t.Errorf("expected 'Y/n:' in prompt, got: %s", out)
		}
	})
}

func TestUserConfirmWithDefault_PromptFormatFalse(t *testing.T) {
	withStdin(t, "\n", func() {
		out := captureStdout(t, func() {
			_, _ = UserConfirmWithDefault(false)
		})
		if !strings.Contains(out, "y/N:") {
			t.Errorf("expected 'y/N:' in prompt, got: %s", out)
		}
	})
}

// --- Prompt format tests ---

func TestUserChoice_PromptFormat(t *testing.T) {
	withStdin(t, "yes\n", func() {
		out := captureStdout(t, func() {
			_, _ = UserChoice("yes", "no", "maybe")
		})
		if !strings.Contains(out, "yes/no/maybe:") {
			t.Errorf("expected 'yes/no/maybe:' in prompt, got: %s", out)
		}
	})
}

func TestUserChoiceWithDefault_PromptFormat(t *testing.T) {
	withStdin(t, "\n", func() {
		out := captureStdout(t, func() {
			_, _ = UserChoiceWithDefault("yes", "no", "maybe")
		})
		// Default should be uppercased, others lowercased.
		if !strings.Contains(out, "YES/no/maybe:") {
			t.Errorf("expected 'YES/no/maybe:' in prompt, got: %s", out)
		}
	})
}
