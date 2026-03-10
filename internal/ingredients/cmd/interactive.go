package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/gogrlx/grlx/v2/internal/log"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

var nc *nats.Conn

func RegisterNatsConn(conn *nats.Conn) {
	nc = conn
}

var envMutex sync.Mutex

func FRun(target pki.KeyManager, cmdRun apitypes.CmdRun) (apitypes.CmdRun, error) {
	topic := "grlx.sprouts." + target.SproutID + ".cmd.run"
	var results apitypes.CmdRun
	b, _ := json.Marshal(cmdRun)
	msg, err := nc.Request(topic, b, time.Second*15+cmdRun.Timeout)
	if err != nil {
		return results, err
	}
	err = json.Unmarshal(msg.Data, &results)
	return results, err
}

func SRun(cmd apitypes.CmdRun) (apitypes.CmdRun, error) {
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

	if cmd.RunAs != "" {
		if err := setRunAs(command, cmd.RunAs); err != nil {
			return cmd, err
		}
	}
	command.Dir = cmd.CWD

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutWriters := []io.Writer{&stdoutBuf}
	stderrWriters := []io.Writer{&stderrBuf}
	// Stream output over NATS for live monitoring when a connection is available.
	if nc != nil && cmd.StreamTopic != "" {
		stdoutWriters = append(stdoutWriters, &natsWriter{conn: nc, topic: cmd.StreamTopic, stream: "stdout"})
		stderrWriters = append(stderrWriters, &natsWriter{conn: nc, topic: cmd.StreamTopic, stream: "stderr"})
	}
	command.Stdout = io.MultiWriter(stdoutWriters...)
	command.Stderr = io.MultiWriter(stderrWriters...)
	timer := time.Now()
	err := command.Run()
	cmd.Duration = time.Since(timer)
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
	}
	cmd.Stdout = stdoutBuf.String()
	cmd.Stderr = stderrBuf.String()
	cmd.ErrCode = command.ProcessState.ExitCode()
	return cmd, err
}
