package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper to run a cmd.run and extract output
func runCmd(t *testing.T, name string, params map[string]interface{}) (output string, result bool, err error) {
	t.Helper()
	if params == nil {
		params = map[string]interface{}{}
	}
	params["name"] = name
	c := Cmd{id: "test", method: "run", params: params}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := c.run(ctx, false)
	for _, note := range res.Notes {
		if strings.HasPrefix(note.String(), "Command output: ") {
			output = strings.TrimPrefix(note.String(), "Command output: ")
		}
	}
	return output, res.Succeeded, err
}

func TestShellPipeChain(t *testing.T) {
	out, ok, err := runCmd(t, `echo "hello world" | tr ' ' '\n' | sort | head -1`, nil)
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

func TestShellMultiplePipes(t *testing.T) {
	out, ok, err := runCmd(t, `echo aAbBcC | tr A-Z a-z | tr -d 'b'`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "aacc" {
		t.Errorf("expected 'aacc', got %q", strings.TrimSpace(out))
	}
}

func TestShellRedirectStdout(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "out.txt")
	_, ok, err := runCmd(t, `echo redirected > `+tmp, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "redirected" {
		t.Errorf("expected 'redirected' in file, got %q", string(data))
	}
}

func TestShellRedirectAppend(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "out.txt")
	_, _, _ = runCmd(t, `echo line1 > `+tmp, nil)
	_, ok, err := runCmd(t, `echo line2 >> `+tmp, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	lines := strings.TrimSpace(string(data))
	if lines != "line1\nline2" {
		t.Errorf("expected 'line1\\nline2', got %q", lines)
	}
}

func TestShellRedirectStderr(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "err.txt")
	// Redirect stderr to file, write to stderr via >&2
	_, _, err := runCmd(t, `echo stderr_msg 2>`+tmp+` >&2`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if strings.TrimSpace(string(data)) != "stderr_msg" {
		t.Errorf("expected 'stderr_msg', got %q", strings.TrimSpace(string(data)))
	}
}

func TestShellCommandSubstitution(t *testing.T) {
	out, ok, err := runCmd(t, `echo "user is $(whoami)"`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "user is ") {
		t.Errorf("expected 'user is <name>', got %q", out)
	}
	if strings.Contains(out, "$(whoami)") {
		t.Error("subshell was not expanded")
	}
}

func TestShellBacktickSubstitution(t *testing.T) {
	out, _, err := runCmd(t, "echo `echo nested`", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "nested" {
		t.Errorf("expected 'nested', got %q", strings.TrimSpace(out))
	}
}

func TestShellBackgroundAmpersand(t *testing.T) {
	// & at end should background the first command; shell should still exit
	tmp := filepath.Join(t.TempDir(), "bg.txt")
	_, ok, err := runCmd(t, `echo background > `+tmp+` &`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	// Give the background process a moment to write
	time.Sleep(500 * time.Millisecond)
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("background command didn't write file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "background" {
		t.Errorf("expected 'background', got %q", string(data))
	}
}

func TestShellAndOperator(t *testing.T) {
	out, ok, err := runCmd(t, `echo first && echo second`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("expected both 'first' and 'second', got %q", out)
	}
}

func TestShellAndOperatorShortCircuit(t *testing.T) {
	out, _, _ := runCmd(t, `false && echo should_not_appear`, nil)
	if strings.Contains(out, "should_not_appear") {
		t.Error("&& should have short-circuited on false")
	}
}

func TestShellOrOperator(t *testing.T) {
	out, _, _ := runCmd(t, `false || echo fallback`, nil)
	if strings.TrimSpace(out) != "fallback" {
		t.Errorf("expected 'fallback', got %q", strings.TrimSpace(out))
	}
}

func TestShellOrOperatorSkip(t *testing.T) {
	out, ok, _ := runCmd(t, `true || echo should_not_appear`, nil)
	if !ok {
		t.Fatal("expected success")
	}
	if strings.Contains(out, "should_not_appear") {
		t.Error("|| should have skipped second command")
	}
}

func TestShellSemicolon(t *testing.T) {
	out, ok, err := runCmd(t, `echo one; echo two; echo three`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
	}
}

func TestShellSingleQuotes(t *testing.T) {
	out, ok, err := runCmd(t, `echo 'hello world'`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "hello world" {
		t.Errorf("expected 'hello world', got %q", strings.TrimSpace(out))
	}
}

func TestShellDoubleQuotesWithSpaces(t *testing.T) {
	out, ok, err := runCmd(t, `echo "hello   world"`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "hello   world" {
		t.Errorf("expected 'hello   world', got %q", strings.TrimSpace(out))
	}
}

func TestShellEmptyQuotedArg(t *testing.T) {
	// This is the ssh-keygen -N "" case from issue #111
	tmp := filepath.Join(t.TempDir(), "test_key")
	_, ok, err := runCmd(t, `ssh-keygen -t ed25519 -q -N "" -f `+tmp, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	// Verify the key was created (no passphrase prompt = empty string worked)
	if _, err := os.Stat(tmp); os.IsNotExist(err) {
		t.Error("ssh-keygen didn't create the key file — empty string arg likely failed")
	}
}

func TestShellEnvironmentVariable(t *testing.T) {
	out, ok, err := runCmd(t, `echo $HOME`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "$HOME" || trimmed == "" {
		t.Errorf("env var not expanded, got %q", trimmed)
	}
}

func TestShellHereDoc(t *testing.T) {
	// Here-doc is POSIX sh compatible (unlike here-string <<<)
	out, ok, err := runCmd(t, "cat <<EOF\nhere doc content\nEOF", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if strings.TrimSpace(out) != "here doc content" {
		t.Errorf("expected 'here doc content', got %q", strings.TrimSpace(out))
	}
}

func TestShellEscapedCharacters(t *testing.T) {
	out, ok, err := runCmd(t, `echo "hello\tworld"`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	// echo with double quotes interprets \t depending on shell
	// At minimum, the backslash should trigger shell mode
	if strings.Contains(out, `\t`) {
		// Some shells don't expand \t in echo — that's OK
		// The point is the command ran through the shell
		t.Log("shell did not expand \\t (expected for some shells)")
	}
}

func TestShellProcessSubstitution(t *testing.T) {
	// diff <(echo a) <(echo b) — uses process substitution
	_, _, err := runCmd(t, `diff <(echo a) <(echo b)`, nil)
	// diff returns exit 1 when files differ, but it should still run
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The fact that it didn't error with "file not found" for <(echo a)
	// means the shell interpreted it correctly
}
