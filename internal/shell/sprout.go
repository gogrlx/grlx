//go:build !windows

package shell

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/log"
)

// SproutSession manages a single interactive shell session on the sprout side.
type SproutSession struct {
	sessionID   string
	nc          *nats.Conn
	ptmx        *os.File
	cmd         *exec.Cmd
	subs        []*nats.Subscription
	done        chan struct{}
	once        sync.Once
	idleTimeout time.Duration
	idleResetCh chan struct{}
}

// HandleShellStart is the NATS handler for grlx.sprouts.<id>.shell.start.
// It spawns a PTY, wires up NATS I/O, and returns session subjects.
func HandleShellStart(nc *nats.Conn, msg *nats.Msg) {
	var req StartRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		respondError(msg, fmt.Errorf("invalid request: %w", err))
		return
	}

	if req.SessionID == "" {
		respondError(msg, fmt.Errorf("session_id is required"))
		return
	}

	shellCmd := req.Shell
	if shellCmd == "" {
		shellCmd = "/bin/sh"
	}

	// Spawn the shell with a PTY.
	cmd := exec.Command(shellCmd)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		respondError(msg, fmt.Errorf("failed to start shell: %w", err))
		return
	}

	// Set initial terminal size.
	if req.Cols > 0 && req.Rows > 0 {
		if resizeErr := pty.Setsize(ptmx, &pty.Winsize{
			Cols: uint16(req.Cols),
			Rows: uint16(req.Rows),
		}); resizeErr != nil {
			log.Errorf("shell: failed to set initial size: %v", resizeErr)
		}
	}

	subjects := Subjects(req.SessionID)

	var idleTimeout time.Duration
	if req.IdleTimeoutSec > 0 {
		idleTimeout = time.Duration(req.IdleTimeoutSec) * time.Second
	}

	session := &SproutSession{
		sessionID:   req.SessionID,
		nc:          nc,
		ptmx:        ptmx,
		cmd:         cmd,
		done:        make(chan struct{}),
		idleTimeout: idleTimeout,
		idleResetCh: make(chan struct{}, 1),
	}

	// Start idle timeout watcher if configured.
	if idleTimeout > 0 {
		go session.idleWatcher(subjects.DoneSubject)
	}

	// Subscribe to input from CLI → write to PTY stdin.
	inputSub, err := nc.Subscribe(subjects.InputSubject, func(m *nats.Msg) {
		session.resetIdle()
		if _, writeErr := ptmx.Write(m.Data); writeErr != nil {
			log.Errorf("shell %s: write to pty failed: %v", req.SessionID, writeErr)
			session.Close(-1, writeErr.Error())
		}
	})
	if err != nil {
		ptmx.Close()
		respondError(msg, fmt.Errorf("failed to subscribe to input: %w", err))
		return
	}
	session.subs = append(session.subs, inputSub)

	// Subscribe to resize messages.
	resizeSub, err := nc.Subscribe(subjects.ResizeSubject, func(m *nats.Msg) {
		var resize ResizeMessage
		if jsonErr := json.Unmarshal(m.Data, &resize); jsonErr != nil {
			return
		}
		if resize.Cols > 0 && resize.Rows > 0 {
			pty.Setsize(ptmx, &pty.Winsize{
				Cols: uint16(resize.Cols),
				Rows: uint16(resize.Rows),
			})
		}
	})
	if err != nil {
		session.cleanup()
		respondError(msg, fmt.Errorf("failed to subscribe to resize: %w", err))
		return
	}
	session.subs = append(session.subs, resizeSub)

	// Read PTY output → publish to CLI.
	go session.readLoop(subjects.OutputSubject)

	// Wait for the process to exit.
	go session.waitLoop(subjects.DoneSubject)

	// Respond with session subjects.
	respData, _ := json.Marshal(subjects)
	msg.Respond(respData)
	log.Debugf("shell: started session %s (shell=%s)", req.SessionID, shellCmd)
}

// resetIdle resets the idle timeout timer. Called on every input message.
func (s *SproutSession) resetIdle() {
	if s.idleTimeout == 0 {
		return
	}
	select {
	case s.idleResetCh <- struct{}{}:
	default:
		// Channel already has a pending reset signal.
	}
}

// idleWatcher monitors for idle timeout and closes the session if no
// input is received within the configured duration.
func (s *SproutSession) idleWatcher(doneSubject string) {
	timer := time.NewTimer(s.idleTimeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			log.Infof("shell %s: idle timeout (%v), closing session", s.sessionID, s.idleTimeout)
			s.Close(-1, "idle timeout")
			done := DoneMessage{
				ExitCode: -1,
				Error:    "idle timeout",
			}
			data, _ := json.Marshal(done)
			s.nc.Publish(doneSubject, data)
			return
		case <-s.idleResetCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(s.idleTimeout)
		case <-s.done:
			return
		}
	}
}

// readLoop reads from the PTY and publishes output to NATS.
func (s *SproutSession) readLoop(outputSubject string) {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			if pubErr := s.nc.Publish(outputSubject, buf[:n]); pubErr != nil {
				log.Errorf("shell %s: publish output failed: %v", s.sessionID, pubErr)
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Errorf("shell %s: pty read error: %v", s.sessionID, err)
			}
			return
		}
	}
}

// waitLoop waits for the shell process to exit and publishes a done message.
func (s *SproutSession) waitLoop(doneSubject string) {
	err := s.cmd.Wait()
	exitCode := 0
	errMsg := ""
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			errMsg = err.Error()
		}
	}
	s.Close(exitCode, errMsg)

	done := DoneMessage{
		ExitCode: exitCode,
		Error:    errMsg,
	}
	data, _ := json.Marshal(done)
	s.nc.Publish(doneSubject, data)
	log.Debugf("shell: session %s exited with code %d", s.sessionID, exitCode)
}

// Close cleans up the session.
func (s *SproutSession) Close(exitCode int, errMsg string) {
	s.once.Do(func() {
		close(s.done)
		s.cleanup()
	})
}

func (s *SproutSession) cleanup() {
	for _, sub := range s.subs {
		sub.Unsubscribe()
	}
	if s.ptmx != nil {
		s.ptmx.Close()
	}
}

func respondError(msg *nats.Msg, err error) {
	resp := struct {
		Error string `json:"error"`
	}{Error: err.Error()}
	data, _ := json.Marshal(resp)
	msg.Respond(data)
}
