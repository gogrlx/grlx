//go:build !windows

package cmd

import (
	"os/exec"
	"os/user"
	"testing"
)

func TestSetRunAsCurrentUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skipf("cannot get current user: %v", err)
	}
	command := exec.Command("echo", "test")
	err = setRunAs(command, u.Username)
	if err != nil {
		t.Fatalf("unexpected error setting runas to current user %q: %v", u.Username, err)
	}
	if command.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	if command.SysProcAttr.Credential == nil {
		t.Fatal("expected Credential to be set")
	}
}

func TestSetRunAsNonexistentUser(t *testing.T) {
	command := exec.Command("echo", "test")
	err := setRunAs(command, "nonexistent_user_xyz_99999")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}
