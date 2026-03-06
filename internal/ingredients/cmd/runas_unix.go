//go:build !windows

package cmd

import (
	"fmt"
	"math"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// setRunAs configures the command to run as a different user on Unix systems.
func setRunAs(command *exec.Cmd, runAs string) error {
	u, err := user.Lookup(runAs)
	if err != nil {
		return err
	}
	uid64, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	if uid64 > math.MaxInt32 {
		return fmt.Errorf("UID %d is invalid", uid64)
	}
	gid64, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	if gid64 > math.MaxInt32 {
		return fmt.Errorf("GID %d is invalid", gid64)
	}
	command.SysProcAttr = &syscall.SysProcAttr{}
	command.SysProcAttr.Credential = &syscall.Credential{
		Uid: uint32(uid64),
		Gid: uint32(gid64),
	}
	return nil
}
