package cmd

// Tests for the SSH picker model are in internal/sshpicker/picker_test.go
// to avoid the package init() in root.go which requires TLS setup.

import "testing"

func TestResolveSSHTarget_BothArgAndCohort(t *testing.T) {
	// Save and restore global flag state.
	old := sshCohort
	defer func() { sshCohort = old }()

	sshCohort = "web-servers"
	_, err := resolveSSHTarget([]string{"sprout-1"})
	if err == nil {
		t.Fatal("expected error when both arg and --cohort are provided")
	}
}

func TestResolveSSHTarget_NeitherArgNorCohort(t *testing.T) {
	old := sshCohort
	defer func() { sshCohort = old }()

	sshCohort = ""
	_, err := resolveSSHTarget(nil)
	if err == nil {
		t.Fatal("expected error when neither arg nor --cohort is provided")
	}
}

func TestResolveSSHTarget_DirectArg(t *testing.T) {
	old := sshCohort
	defer func() { sshCohort = old }()

	sshCohort = ""
	id, err := resolveSSHTarget([]string{"my-sprout"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-sprout" {
		t.Errorf("got %q, want %q", id, "my-sprout")
	}
}
