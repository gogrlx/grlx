package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gogrlx/grlx/types"
)

func (c Cmd) run(ctx context.Context, test bool) (types.Result, error) {
	var result types.Result
	var err error

	cmd, ok := c.params["name"].(string)
	if !ok {
		result.Succeeded = false
		result.Failed = true
		return result, errors.New("invalid command; name must be a string")
	}
	splitCmd := strings.Split(cmd, " ")
	if len(splitCmd) == 0 {
		return result, errors.New("invalid command; name must not be empty")
	}
	args := splitCmd[1:]

	runas := ""
	path := ""
	cwd := ""
	env := []string{}
	timeout := ""
	if runasInter, ok := c.params["runas"]; ok {
		runas, ok = runasInter.(string)
		if !ok {
			return result, fmt.Errorf("invalid runas %v; must be a string", runasInter)
		}
	}
	if pathInter, ok := c.params["path"]; ok {
		path, ok = pathInter.(string)
		if !ok {
			return result, fmt.Errorf("invalid path %v; must be a string", pathInter)
		}
	}
	if cwdInter, ok := c.params["cwd"]; ok {
		cwd, ok = cwdInter.(string)
		if !ok {
			return result, fmt.Errorf("invalid cwd %v; must be a string", cwdInter)
		}
	}
	if envInter, ok := c.params["env"]; ok {
		env, ok = envInter.([]string)
		if !ok {
			return result, fmt.Errorf("invalid env %v; must be a string slice like `k=v`", envInter)
		}
	}
	if timeoutInter, ok := c.params["timeout"]; ok {
		timeout, ok = timeoutInter.(string)
		if !ok {
			return result, fmt.Errorf("invalid timeout %v; must be a string", timeoutInter)
		}
	}
	// sanity check env vars
	envVars := map[string]string{}
	for _, envVar := range env {
		sp := strings.Split(envVar, "=")
		if len(sp) != 2 {
			return result, fmt.Errorf("invalid env var %s; vars must be key=value pairs", envVar)
		}
		envVars[sp[0]] = sp[1]
	}
	var command *exec.Cmd
	if timeout != "" {
		ttimeout, parseErr := time.ParseDuration(timeout)
		if parseErr != nil {
			result.Succeeded = false
			result.Failed = true
			result.Changed = false
			result.Notes = append(result.Notes, types.SimpleNote(fmt.Sprintf("invalid timeout %s; must be a valid duration", timeout)))
			return result, errors.Join(parseErr, fmt.Errorf("invalid timeout %s; must be a valid duration", timeout))
		}
		timeoutCTX, cancel := context.WithTimeout(ctx, ttimeout)
		defer cancel()
		command = exec.CommandContext(timeoutCTX, splitCmd[0], args...)
	} else {
		command = exec.CommandContext(ctx, splitCmd[0], args...)
	}
	if runas != "" && runtime.GOOS != "windows" {
		u, lookupErr := user.Lookup(runas)
		if lookupErr != nil {
			return result, errors.Join(lookupErr, fmt.Errorf("invalid user %s; user must exist", runas))
		}
		uid64, strNameErr := strconv.Atoi(u.Uid)
		if strNameErr != nil {
			return result, errors.Join(strNameErr, fmt.Errorf("invalid user %s; user must exist", runas))
		}
		if uid64 > math.MaxInt32 {
			return result, fmt.Errorf("UID %d is invalid", uid64)
		}
		uid := uint32(uid64)
		command.SysProcAttr = &syscall.SysProcAttr{}
		command.SysProcAttr.Credential = &syscall.Credential{Uid: uid}
	}
	if path != "" {
		command.Path = path
	}
	if cwd != "" {
		command.Dir = cwd
	}
	if len(envVars) > 0 {
		command.Env = []string{}
		command.Env = append(command.Env, env...)
	}
	if test {
		result.Notes = append(result.Notes,
			types.SimpleNote("Command would have been run"))
		return result, nil
	}

	out, err := command.CombinedOutput()
	result.Notes = append(result.Notes,
		types.SimpleNote(fmt.Sprintf("Command output: %s", string(out))),
	)

	if err != nil {
		result.Notes = append(result.Notes,
			types.SimpleNote(fmt.Sprintf("Command failed: %s", err.Error())))
	}
	if command.ProcessState.ExitCode() != 0 {
		result.Succeeded = false
		result.Failed = true
	} else {
		result.Succeeded = true
		result.Failed = false
	}
	return result, nil
}
