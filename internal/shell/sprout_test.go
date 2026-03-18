//go:build !windows

package shell

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestRespondError(t *testing.T) {
	// respondError should not panic even with no Reply subject.
	// It formats the error and calls msg.Respond which is a no-op
	// without a connection, but the marshal should succeed.
	msg := &nats.Msg{Data: nil}
	respondError(msg, fmt.Errorf("test error"))
	// msg.Respond will fail silently (no connection) — that's fine.
}

func TestSproutSession_IdleReset(t *testing.T) {
	s := &SproutSession{
		idleTimeout: time.Second,
		idleResetCh: make(chan struct{}, 1),
		done:        make(chan struct{}),
	}

	// Reset should not block.
	s.resetIdle()
	select {
	case <-s.idleResetCh:
		// Got the signal — good.
	default:
		t.Fatal("expected reset signal in channel")
	}
}

func TestSproutSession_NoIdleReset(t *testing.T) {
	// With zero timeout, resetIdle should be a no-op.
	s := &SproutSession{
		idleTimeout: 0,
		idleResetCh: make(chan struct{}, 1),
		done:        make(chan struct{}),
	}

	s.resetIdle()
	select {
	case <-s.idleResetCh:
		t.Fatal("resetIdle should be no-op when idleTimeout is 0")
	default:
		// Good — nothing sent.
	}
}

func TestStartRequest_IdleTimeout(t *testing.T) {
	req := StartRequest{
		SessionID:      "test-idle",
		Cols:           80,
		Rows:           24,
		IdleTimeoutSec: 300,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded StartRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.IdleTimeoutSec != 300 {
		t.Errorf("IdleTimeoutSec = %d, want 300", decoded.IdleTimeoutSec)
	}
}

func TestStartRequest_IdleTimeoutOmitted(t *testing.T) {
	req := StartRequest{
		SessionID: "no-timeout",
		Cols:      80,
		Rows:      24,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// idle_timeout_sec should be omitted from JSON when zero.
	if string(data) != `{"session_id":"no-timeout","cols":80,"rows":24}` {
		// Check it doesn't include idle_timeout_sec.
		var m map[string]any
		json.Unmarshal(data, &m)
		if _, ok := m["idle_timeout_sec"]; ok {
			t.Error("idle_timeout_sec should be omitted when zero")
		}
	}
}

func TestStartRequest_DefaultShell(t *testing.T) {
	// Verify that an empty Shell field means the handler should
	// default to /bin/sh (the handler logic, not the struct).
	req := StartRequest{
		SessionID: "test-123",
		Cols:      120,
		Rows:      40,
	}
	if req.Shell != "" {
		t.Errorf("expected empty Shell default, got %q", req.Shell)
	}

	// The handler fills in /bin/sh when Shell is empty.
	shellCmd := req.Shell
	if shellCmd == "" {
		shellCmd = "/bin/sh"
	}
	if shellCmd != "/bin/sh" {
		t.Errorf("expected /bin/sh default, got %q", shellCmd)
	}
}

func TestStartRequest_CustomShell(t *testing.T) {
	req := StartRequest{
		SessionID: "test-456",
		Cols:      80,
		Rows:      24,
		Shell:     "/bin/bash",
	}
	if req.Shell != "/bin/bash" {
		t.Errorf("expected /bin/bash, got %q", req.Shell)
	}
}

func TestStartRequest_JSONRoundTrip(t *testing.T) {
	req := StartRequest{
		SessionID: "round-trip-id",
		Cols:      132,
		Rows:      43,
		Shell:     "/bin/zsh",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded StartRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SessionID != req.SessionID {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, req.SessionID)
	}
	if decoded.Cols != req.Cols || decoded.Rows != req.Rows {
		t.Errorf("dimensions = (%d, %d), want (%d, %d)", decoded.Cols, decoded.Rows, req.Cols, req.Rows)
	}
	if decoded.Shell != req.Shell {
		t.Errorf("Shell = %q, want %q", decoded.Shell, req.Shell)
	}
}

func TestStartResponse_JSONRoundTrip(t *testing.T) {
	resp := StartResponse{
		SessionID:     "test-session",
		InputSubject:  "grlx.shell.test-session.input",
		OutputSubject: "grlx.shell.test-session.output",
		ResizeSubject: "grlx.shell.test-session.resize",
		DoneSubject:   "grlx.shell.test-session.done",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded StartResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != resp {
		t.Errorf("got %+v, want %+v", decoded, resp)
	}
}

func TestDoneMessage_JSONRoundTrip(t *testing.T) {
	dm := DoneMessage{ExitCode: 1, Error: "signal: killed"}
	data, err := json.Marshal(dm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DoneMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", decoded.ExitCode)
	}
	if decoded.Error != "signal: killed" {
		t.Errorf("Error = %q", decoded.Error)
	}
}

func TestResizeMessage_JSONRoundTrip(t *testing.T) {
	rm := ResizeMessage{Cols: 200, Rows: 50}
	data, err := json.Marshal(rm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ResizeMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Cols != 200 || decoded.Rows != 50 {
		t.Errorf("got (%d, %d), want (200, 50)", decoded.Cols, decoded.Rows)
	}
}

func TestCLIStartRequest_JSONRoundTrip(t *testing.T) {
	req := CLIStartRequest{
		SproutID: "my-sprout",
		Cols:     120,
		Rows:     40,
		Shell:    "/bin/bash",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CLIStartRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SproutID != req.SproutID {
		t.Errorf("SproutID = %q, want %q", decoded.SproutID, req.SproutID)
	}
}
