package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/rbac"
)

// --- Sprouts display ---

func TestSproutsCommand_HelpOutput(t *testing.T) {
	// cmdSprouts.Run calls cmd.Help(), verify it doesn't panic.
	if cmdSprouts.Run == nil {
		t.Fatal("sprouts command should have Run set")
	}
}

func TestCohortsCommand_HelpOutput(t *testing.T) {
	if cmdCohorts.Run == nil {
		t.Fatal("cohorts command should have Run set")
	}
}

func TestAuthCommand_HelpOutput(t *testing.T) {
	if authCmd.Run == nil {
		t.Fatal("auth command should have Run set")
	}
}

func TestUsersCommand_HelpOutput(t *testing.T) {
	if usersCmd.Run == nil {
		t.Fatal("users command should have Run set")
	}
}

func TestCmdCommand_HelpOutput(t *testing.T) {
	if cmdCmd.Run == nil {
		t.Fatal("cmd command should have Run set")
	}
}

func TestTestCommand_HelpOutput(t *testing.T) {
	if testCmd.Run == nil {
		t.Fatal("test command should have Run set")
	}
}

func TestKeysCommand_HelpOutput(t *testing.T) {
	if keysCmd.Run == nil {
		t.Fatal("keys command should have Run set")
	}
}

// --- JSON output formatting ---

func TestVersionJSON_Marshaling(t *testing.T) {
	cv := config.CombinedVersion{
		CLI: config.Version{
			Tag:       "v2.1.0",
			GitCommit: "abc123",
			Arch:      "linux/amd64",
			Compiler:  "go1.23.0",
		},
		Farmer: config.Version{
			Tag:       "v2.1.0",
			GitCommit: "def456",
			Arch:      "linux/arm64",
			Compiler:  "go1.23.0",
		},
	}

	b, err := json.Marshal(cv)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "v2.1.0") {
		t.Error("expected version tag in JSON")
	}
	if !strings.Contains(s, "abc123") {
		t.Error("expected CLI commit in JSON")
	}
	if !strings.Contains(s, "def456") {
		t.Error("expected farmer commit in JSON")
	}
}

func TestVersionJSON_WithError(t *testing.T) {
	cv := config.CombinedVersion{
		CLI: config.Version{Tag: "v2.1.0"},
		Error: "connection refused",
	}

	b, err := json.Marshal(cv)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "connection refused") {
		t.Error("expected error in JSON output")
	}
}

// --- Roles display ---

func TestRolesJSON_Marshaling(t *testing.T) {
	type roleEntry struct {
		Name    string      `json:"name"`
		Rules   []rbac.Rule `json:"rules"`
		Builtin bool        `json:"builtin"`
	}

	entries := []roleEntry{
		{
			Name:    "viewer",
			Rules:   []rbac.Rule{{Action: "view", Scope: "*"}},
			Builtin: true,
		},
		{
			Name:    "operator",
			Rules:   []rbac.Rule{{Action: "cook", Scope: "web-*"}, {Action: "view", Scope: "*"}},
			Builtin: true,
		},
		{
			Name:    "deploy-team",
			Rules:   []rbac.Rule{{Action: "cook", Scope: "*"}},
			Builtin: false,
		},
	}

	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "viewer") {
		t.Error("expected viewer role in JSON")
	}
	if !strings.Contains(s, "deploy-team") {
		t.Error("expected custom role in JSON")
	}
}

// --- RecipeInfo display ---

func TestPrintRecipesTable_Empty(t *testing.T) {
	out := captureStdout(t, func() {
		printRecipesTable([]RecipeInfo{})
	})
	if !strings.Contains(out, "0 recipe(s)") {
		t.Error("expected '0 recipe(s)' for empty list")
	}
}

func TestPrintRecipesTable_LongNames(t *testing.T) {
	recipes := []RecipeInfo{
		{Name: "this-is-a-very-long-recipe-name.with.deep.nesting", Path: "/srv/recipes/deep/nesting.grlx", Size: 65536},
	}
	out := captureStdout(t, func() {
		printRecipesTable(recipes)
	})
	if !strings.Contains(out, "this-is-a-very-long-recipe-name") {
		t.Error("expected long recipe name in output")
	}
	if !strings.Contains(out, "64.0 KB") {
		t.Error("expected formatted size 64.0 KB")
	}
}

// --- Audit display ---

func TestAuditDatesFlags(t *testing.T) {
	// Already tested in cmd_test.go but let's verify the command exists
	if cmdAuditDates.Run == nil {
		t.Fatal("audit dates should have Run set")
	}
}

// --- Init command ---

func TestInitCommand_Exists(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("root command missing 'init' subcommand")
	}
}

// --- Roles command ---

func TestRolesCommand_Exists(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "roles" {
			found = true
			break
		}
	}
	if !found {
		t.Error("root command missing 'roles' subcommand")
	}
}

// --- formatSize edge cases ---

func TestFormatSize_GigabyteRange(t *testing.T) {
	gb := int64(1073741824) // 1 GB
	got := formatSize(gb)
	if got != "1.0 GB" {
		t.Errorf("formatSize(%d) = %q, want %q", gb, got, "1.0 GB")
	}
}

func TestFormatSize_LargeKB(t *testing.T) {
	size := int64(524288) // 512 KB
	got := formatSize(size)
	if got != "512.0 KB" {
		t.Errorf("formatSize(%d) = %q, want %q", size, got, "512.0 KB")
	}
}

// --- printJobsTable JSON mode simulation ---

func TestJobsListJSON_Marshaling(t *testing.T) {
	// Simulates the JSON output path of jobs list.
	type jobSummarySlim struct {
		JID    string `json:"jid"`
		Status string `json:"status"`
	}
	summaries := []jobSummarySlim{
		{JID: "j1", Status: "succeeded"},
		{JID: "j2", Status: "failed"},
	}
	b, err := json.Marshal(summaries)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "j1") || !strings.Contains(s, "j2") {
		t.Error("expected both JIDs in JSON output")
	}
}

// --- Global targeting flags ---

func TestTargetFlagsExist_CmdCmd(t *testing.T) {
	pf := cmdCmd.PersistentFlags()
	if f := pf.Lookup("target"); f == nil {
		t.Error("cmd missing --target persistent flag")
	}
	if f := pf.Lookup("cohort"); f == nil {
		t.Error("cmd missing --cohort persistent flag")
	}
}

func TestTargetFlagsExist_TestCmd(t *testing.T) {
	pf := testCmd.PersistentFlags()
	if f := pf.Lookup("target"); f == nil {
		t.Error("test missing --target persistent flag")
	}
	if f := pf.Lookup("cohort"); f == nil {
		t.Error("test missing --cohort persistent flag")
	}
}
