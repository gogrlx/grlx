//go:build windows

package cmd

import (
	"fmt"
	"os/exec"
)

// setRunAs is not supported on Windows.
func setRunAs(command *exec.Cmd, runAs string) error {
	return fmt.Errorf("RunAs is not supported on Windows")
}
