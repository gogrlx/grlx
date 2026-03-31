package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// --- resolveEffectiveTarget tests ---

func TestResolveEffectiveTarget_TargetOnly_Simple(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = "web-1"
	cohortTarget = ""
	got, err := resolveEffectiveTarget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "web-1" {
		t.Errorf("got %q, want %q", got, "web-1")
	}
}

func TestResolveEffectiveTarget_TargetOnly_Regex(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = "web-.*"
	cohortTarget = ""
	got, err := resolveEffectiveTarget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "web-.*" {
		t.Errorf("got %q, want %q", got, "web-.*")
	}
}

func TestResolveEffectiveTarget_TargetOnly_CommaSeparated(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = "web-1,web-2,db-1"
	cohortTarget = ""
	got, err := resolveEffectiveTarget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "web-1,web-2,db-1" {
		t.Errorf("got %q, want %q", got, "web-1,web-2,db-1")
	}
}

func TestResolveEffectiveTarget_NeitherSet_Error(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = ""
	cohortTarget = ""
	_, err := resolveEffectiveTarget()
	if err == nil {
		t.Fatal("expected error when neither flag is set")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention 'required': %v", err)
	}
}

func TestResolveEffectiveTarget_BothSet_Error(t *testing.T) {
	oldS, oldC := sproutTarget, cohortTarget
	defer func() { sproutTarget = oldS; cohortTarget = oldC }()

	sproutTarget = "web-1"
	cohortTarget = "web-servers"
	_, err := resolveEffectiveTarget()
	if err == nil {
		t.Fatal("expected error when both flags are set")
	}
	if !strings.Contains(err.Error(), "cannot use both") {
		t.Errorf("error should mention 'cannot use both': %v", err)
	}
}

// --- addTargetFlags tests ---

func TestAddTargetFlags_RegistersFlags(t *testing.T) {
	// cook, cmd, and test all use addTargetFlags — verify the flags exist.
	cmds := map[string]*cobra.Command{
		"cook": cmdCook,
		"cmd":  cmdCmd,
		"test": testCmd,
	}
	for name, c := range cmds {
		flags := c.PersistentFlags()
		if f := flags.Lookup("target"); f == nil {
			t.Errorf("%s missing --target (-T) persistent flag", name)
		}
		if f := flags.Lookup("cohort"); f == nil {
			t.Errorf("%s missing --cohort (-C) persistent flag", name)
		}
	}
}

func TestCookCommand_HasCohortFlag(t *testing.T) {
	f := cmdCook.PersistentFlags().Lookup("cohort")
	if f == nil {
		t.Fatal("cook command should have --cohort flag")
	}
	if f.Shorthand != "C" {
		t.Errorf("cook --cohort shorthand = %q, want 'C'", f.Shorthand)
	}
}

func TestCmdCommand_HasCohortFlag(t *testing.T) {
	f := cmdCmd.PersistentFlags().Lookup("cohort")
	if f == nil {
		t.Fatal("cmd command should have --cohort flag")
	}
	if f.Shorthand != "C" {
		t.Errorf("cmd --cohort shorthand = %q, want 'C'", f.Shorthand)
	}
}

func TestTestCommand_HasCohortFlag(t *testing.T) {
	f := testCmd.PersistentFlags().Lookup("cohort")
	if f == nil {
		t.Fatal("test command should have --cohort flag")
	}
	if f.Shorthand != "C" {
		t.Errorf("test --cohort shorthand = %q, want 'C'", f.Shorthand)
	}
}

func TestSSHCommand_HasCohortFlag(t *testing.T) {
	f := sshCmd.Flags().Lookup("cohort")
	if f == nil {
		t.Fatal("ssh command should have --cohort flag")
	}
	if f.Shorthand != "C" {
		t.Errorf("ssh --cohort shorthand = %q, want 'C'", f.Shorthand)
	}
}

func TestSproutsListCommand_HasCohortFlag(t *testing.T) {
	f := cmdSproutsList.Flags().Lookup("cohort")
	if f == nil {
		t.Fatal("sprouts list command should have --cohort flag")
	}
	if f.Shorthand != "C" {
		t.Errorf("sprouts list --cohort shorthand = %q, want 'C'", f.Shorthand)
	}
}

func TestJobsListCommand_HasCohortFlag(t *testing.T) {
	f := cmdJobsList.Flags().Lookup("cohort")
	if f == nil {
		t.Fatal("jobs list command should have --cohort flag")
	}
	if f.Shorthand != "C" {
		t.Errorf("jobs list --cohort shorthand = %q, want 'C'", f.Shorthand)
	}
}

// --- cook Use string documents -C ---

func TestCookUseString_DocumentsCohort(t *testing.T) {
	if !strings.Contains(cmdCook.Use, "-C") {
		t.Error("cook Use string should mention -C flag")
	}
	if !strings.Contains(cmdCook.Use, "cohort") {
		t.Error("cook Use string should mention 'cohort'")
	}
}

// SSH targeting tests are in ssh_test.go to avoid redeclaration.
