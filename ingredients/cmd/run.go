package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	. "github.com/gogrlx/grlx/types"
	"github.com/taigrr/log-socket/log"
)

var envMutex sync.Mutex

// TODO allow selector to be more than an ID
func FRun(target KeyManager, cmdRun CmdRun) (CmdRun, error) {
	topic := "grlx.sprouts." + target.SproutID + ".cmd.run"
	var results CmdRun
	err := ec.Request(topic, cmdRun, &results, time.Second*15+cmdRun.Duration)
	return results, err
}

func SRun(cmd CmdRun) (CmdRun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()
	envMutex.Lock()
	osPath := os.Getenv("PATH")
	newPath := ""
	if val, ok := cmd.Env["PATH"]; cmd.Path == "" && (!ok || (ok && val == "")) {
		_, err := exec.LookPath(cmd.Command)
		if err != nil {
			envMutex.Unlock()
			cmd.Error = err
			return cmd, err
		}
	} else {
		if cmd.Path != "" {
			newPath += cmd.Path + string(os.PathListSeparator)
		}
		if ok && val != "" {
			newPath += val + string(os.PathListSeparator)
		}
	}
	os.Setenv("PATH", newPath+osPath)
	command := exec.CommandContext(ctx, cmd.Command, cmd.Args...)
	os.Setenv("PATH", osPath)
	env := os.Environ()
	envMutex.Unlock()
	for key, val := range cmd.Env {
		env = append(env, key+"="+val)
	}
	command.Env = env

	var uid uint32
	//TODO fix for windows support
	if cmd.RunAs != "" && runtime.GOOS != "windows" {
		u, err := user.Lookup(cmd.RunAs)
		if err != nil {
			return cmd, err
		}
		uid64, err := strconv.Atoi(u.Uid)
		uid = uint32(uid64)
		if err != nil {
			return cmd, err
		}
		command.SysProcAttr = &syscall.SysProcAttr{}
		command.SysProcAttr.Credential = &syscall.Credential{Uid: uid}
	}
	command.Dir = cmd.CWD

	//TODO replace os.Stdout/err here with writes to websocket to get live returnable data
	var stdoutBuf, stderrBuf bytes.Buffer
	command.Stdout = io.MultiWriter(&stdoutBuf) //, os.Stdout)
	command.Stderr = io.MultiWriter(&stderrBuf) //, os.Stderr)
	timer := time.Now()
	err := command.Run()
	cmd.Duration = time.Now().Sub(timer)
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
	}
	cmd.Stdout = stdoutBuf.String()
	cmd.Stderr = stderrBuf.String()
	cmd.ErrCode = command.ProcessState.ExitCode()
	return cmd, nil
}
