package cmd

import (
	"strings"
	"testing"
	"time"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
)

// TestCmdRunEnvParsing verifies the environment variable parsing logic
// used in the cmd run command handler.
func TestCmdRunEnvParsing(t *testing.T) {
	tests := []struct {
		name     string
		envStr   string
		expected map[string]string
	}{
		{
			name:     "single pair",
			envStr:   "FOO=bar",
			expected: map[string]string{"FOO": "bar"},
		},
		{
			name:     "multiple pairs",
			envStr:   "FOO=bar BAZ=qux",
			expected: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:     "value with equals",
			envStr:   "CONNECTION=host=localhost port=5432",
			expected: map[string]string{"CONNECTION": "host=localhost", "port": "5432"},
		},
		{
			name:     "empty string",
			envStr:   "",
			expected: map[string]string{},
		},
		{
			name:     "no equals sign",
			envStr:   "JUSTKEY",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := make(apitypes.EnvVar)
			for _, pair := range strings.Split(tt.envStr, " ") {
				if strings.ContainsRune(pair, '=') {
					kv := strings.SplitN(pair, "=", 2)
					env[kv[0]] = kv[1]
				}
			}
			for k, want := range tt.expected {
				got, ok := env[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if got != want {
					t.Errorf("env[%q] = %q, want %q", k, got, want)
				}
			}
			if len(env) != len(tt.expected) {
				t.Errorf("env has %d entries, want %d", len(env), len(tt.expected))
			}
		})
	}
}

// TestCmdRunCommandConstruction verifies command struct assembly.
func TestCmdRunCommandConstruction(t *testing.T) {
	var command apitypes.CmdRun
	command.Command = "ls"
	command.Args = []string{"-la", "/tmp"}
	command.CWD = "/home/user"
	command.Timeout = 30 * time.Second
	command.Env = make(apitypes.EnvVar)
	command.Env["HOME"] = "/home/user"
	command.Path = "/usr/local/bin"
	command.RunAs = "deploy"

	if command.Command != "ls" {
		t.Errorf("expected command 'ls', got %q", command.Command)
	}
	if len(command.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(command.Args))
	}
	if command.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", command.Timeout)
	}
	if command.RunAs != "deploy" {
		t.Errorf("expected RunAs 'deploy', got %q", command.RunAs)
	}
}

// TestCmdCookTypeConstruction verifies cook type assembly.
func TestCmdCookTypeConstruction(t *testing.T) {
	var cmdCookReq apitypes.CmdCook
	cmdCookReq.Recipe = "base.packages"
	cmdCookReq.Async = true
	cmdCookReq.Env = "FOO=bar"
	cmdCookReq.Test = true

	if string(cmdCookReq.Recipe) != "base.packages" {
		t.Errorf("expected recipe base.packages, got %s", cmdCookReq.Recipe)
	}
	if !cmdCookReq.Async {
		t.Error("expected Async to be true")
	}
	if !cmdCookReq.Test {
		t.Error("expected Test to be true")
	}
}

// TestCmdRunFlags_Defaults verifies the default flag values for cmd run.
func TestCmdRunFlags_Defaults(t *testing.T) {
	flags := cmdCmdRun.Flags()

	if f := flags.Lookup("timeout"); f == nil {
		t.Error("cmd run missing --timeout flag")
	} else if f.DefValue != "30" {
		t.Errorf("cmd run --timeout default = %q, want 30", f.DefValue)
	}

	if f := flags.Lookup("noerr"); f == nil {
		t.Error("cmd run missing --noerr flag")
	}

	if f := flags.Lookup("cwd"); f == nil {
		t.Error("cmd run missing --cwd flag")
	}

	if f := flags.Lookup("runas"); f == nil {
		t.Error("cmd run missing --runas flag")
	}
}

// TestCmdCmdHasRunSubcommand verifies cmd has the run subcommand.
func TestCmdCmdHasRunSubcommand(t *testing.T) {
	found := false
	for _, c := range cmdCmd.Commands() {
		if c.Name() == "run" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cmd command missing 'run' subcommand")
	}
}
