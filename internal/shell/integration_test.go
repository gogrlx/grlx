//go:build !windows

package shell

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// startTestNATS starts an embedded NATS server for testing and returns
// a connected client. The server and client are cleaned up when the
// test completes.
func startTestNATS(t *testing.T) *nats.Conn {
	t.Helper()
	opts := &natsserver.Options{
		Host: "127.0.0.1",
		Port: -1, // random port
	}
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create NATS server: %v", err)
	}
	srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(srv.Shutdown)

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("failed to connect to NATS: %v", err)
	}
	t.Cleanup(nc.Close)

	return nc
}

func TestHandleShellStart_Success(t *testing.T) {
	nc := startTestNATS(t)

	// Subscribe to handle shell start on a test sprout.
	sub, err := nc.Subscribe("test.shell.start", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "test-session-1",
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.start", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	if err := json.Unmarshal(resp.Data, &startResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if startResp.SessionID != "test-session-1" {
		t.Errorf("SessionID = %q, want %q", startResp.SessionID, "test-session-1")
	}
	if startResp.InputSubject == "" {
		t.Error("InputSubject should not be empty")
	}
	if startResp.OutputSubject == "" {
		t.Error("OutputSubject should not be empty")
	}
	if startResp.DoneSubject == "" {
		t.Error("DoneSubject should not be empty")
	}

	// Send "exit\n" to the shell to close it cleanly.
	nc.Publish(startResp.InputSubject, []byte("exit\n"))
	nc.Flush()

	// Wait for the done message.
	doneSub, err := nc.SubscribeSync(startResp.DoneSubject)
	if err != nil {
		t.Fatalf("subscribe done: %v", err)
	}
	defer doneSub.Unsubscribe()

	doneMsg, err := doneSub.NextMsg(5 * time.Second)
	if err != nil {
		// The shell might have already exited before we subscribed.
		// That's OK — the test verifies the session started.
		t.Logf("done message not received (shell may have exited before subscribe): %v", err)
		return
	}

	var done DoneMessage
	if err := json.Unmarshal(doneMsg.Data, &done); err != nil {
		t.Fatalf("unmarshal done: %v", err)
	}
	// exit code 0 is normal.
	if done.ExitCode != 0 {
		t.Logf("exit code = %d, error = %q (may vary by shell)", done.ExitCode, done.Error)
	}
}

func TestHandleShellStart_InvalidJSON(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.invalid", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	resp, err := nc.Request("test.shell.invalid", []byte("not json"), 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Data, &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected error response for invalid JSON")
	}
	if !strings.Contains(errResp.Error, "invalid request") {
		t.Errorf("error = %q, want to contain 'invalid request'", errResp.Error)
	}
}

func TestHandleShellStart_EmptySessionID(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.nosession", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "", // empty — should fail
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.nosession", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Data, &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(errResp.Error, "session_id is required") {
		t.Errorf("error = %q, want 'session_id is required'", errResp.Error)
	}
}

func TestHandleShellStart_CustomShell(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.custom", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "custom-shell-test",
		Cols:      120,
		Rows:      40,
		Shell:     "/bin/sh",
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.custom", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	if err := json.Unmarshal(resp.Data, &startResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if startResp.SessionID != "custom-shell-test" {
		t.Errorf("SessionID = %q, want %q", startResp.SessionID, "custom-shell-test")
	}

	// Clean up the session.
	nc.Publish(startResp.InputSubject, []byte("exit\n"))
	nc.Flush()
	time.Sleep(500 * time.Millisecond)
}

func TestHandleShellStart_IOExchange(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.io", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "io-test",
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.io", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	// Collect output.
	var outputMu sync.Mutex
	var outputBuf []byte
	outputSub, err := nc.Subscribe(startResp.OutputSubject, func(msg *nats.Msg) {
		outputMu.Lock()
		outputBuf = append(outputBuf, msg.Data...)
		outputMu.Unlock()
	})
	if err != nil {
		t.Fatalf("subscribe output: %v", err)
	}
	defer outputSub.Unsubscribe()

	// Send a command that produces known output.
	nc.Publish(startResp.InputSubject, []byte("echo GRLX_TEST_MARKER\n"))
	nc.Flush()

	// Wait for the marker to appear in output.
	deadline := time.After(5 * time.Second)
	for {
		outputMu.Lock()
		got := string(outputBuf)
		outputMu.Unlock()
		if strings.Contains(got, "GRLX_TEST_MARKER") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for output, got: %q", got)
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Clean up.
	nc.Publish(startResp.InputSubject, []byte("exit\n"))
	nc.Flush()
	time.Sleep(500 * time.Millisecond)
}

func TestHandleShellStart_Resize(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.resize", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "resize-test",
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.resize", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	// Send resize messages — should not crash.
	resizeMsg := ResizeMessage{Cols: 200, Rows: 50}
	resizeData, _ := json.Marshal(resizeMsg)
	nc.Publish(startResp.ResizeSubject, resizeData)
	nc.Flush()

	// Send invalid resize (zero dimensions) — should be ignored gracefully.
	zeroResize := ResizeMessage{Cols: 0, Rows: 0}
	zeroData, _ := json.Marshal(zeroResize)
	nc.Publish(startResp.ResizeSubject, zeroData)
	nc.Flush()

	// Send invalid JSON to resize — should be ignored.
	nc.Publish(startResp.ResizeSubject, []byte("not json"))
	nc.Flush()

	time.Sleep(200 * time.Millisecond)

	// Clean up.
	nc.Publish(startResp.InputSubject, []byte("exit\n"))
	nc.Flush()
	time.Sleep(500 * time.Millisecond)
}

func TestHandleShellStart_DoneOnExit(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.done", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "done-test",
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.done", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	// Subscribe to done BEFORE exiting the shell.
	doneSub, err := nc.SubscribeSync(startResp.DoneSubject)
	if err != nil {
		t.Fatalf("subscribe done: %v", err)
	}
	defer doneSub.Unsubscribe()

	// Exit the shell.
	nc.Publish(startResp.InputSubject, []byte("exit 0\n"))
	nc.Flush()

	doneMsg, err := doneSub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("timed out waiting for done message: %v", err)
	}

	var done DoneMessage
	if err := json.Unmarshal(doneMsg.Data, &done); err != nil {
		t.Fatalf("unmarshal done: %v", err)
	}
	if done.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", done.ExitCode)
	}
}

func TestHandleShellStart_NonZeroExit(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.nonzero", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "nonzero-test",
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.nonzero", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	doneSub, err := nc.SubscribeSync(startResp.DoneSubject)
	if err != nil {
		t.Fatalf("subscribe done: %v", err)
	}
	defer doneSub.Unsubscribe()

	// Exit with non-zero code.
	nc.Publish(startResp.InputSubject, []byte("exit 42\n"))
	nc.Flush()

	doneMsg, err := doneSub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("timed out waiting for done message: %v", err)
	}

	var done DoneMessage
	json.Unmarshal(doneMsg.Data, &done)
	if done.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", done.ExitCode)
	}
}

func TestHandleShellStart_IdleTimeout(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.idle", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID:      "idle-timeout-test",
		Cols:           80,
		Rows:           24,
		IdleTimeoutSec: 2, // 2 second timeout
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.idle", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	doneSub, err := nc.SubscribeSync(startResp.DoneSubject)
	if err != nil {
		t.Fatalf("subscribe done: %v", err)
	}
	defer doneSub.Unsubscribe()

	// Don't send any input — wait for idle timeout.
	doneMsg, err := doneSub.NextMsg(10 * time.Second)
	if err != nil {
		t.Fatalf("timed out waiting for idle timeout done message: %v", err)
	}

	var done DoneMessage
	json.Unmarshal(doneMsg.Data, &done)
	if done.Error != "idle timeout" {
		t.Errorf("error = %q, want 'idle timeout'", done.Error)
	}
	if done.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1", done.ExitCode)
	}
}

func TestHandleShellStart_IdleTimeoutReset(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.idlereset", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID:      "idle-reset-test",
		Cols:           80,
		Rows:           24,
		IdleTimeoutSec: 3, // 3 second timeout
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.idlereset", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	// Send input every 1.5s to keep resetting the idle timer.
	// After 3 resets (4.5s total), stop and let it time out.
	for range 3 {
		time.Sleep(1500 * time.Millisecond)
		nc.Publish(startResp.InputSubject, []byte("\n"))
		nc.Flush()
	}

	doneSub, err := nc.SubscribeSync(startResp.DoneSubject)
	if err != nil {
		t.Fatalf("subscribe done: %v", err)
	}
	defer doneSub.Unsubscribe()

	// Now wait for the idle timeout (should fire ~3s after last input).
	doneMsg, err := doneSub.NextMsg(10 * time.Second)
	if err != nil {
		t.Fatalf("timed out waiting for idle timeout: %v", err)
	}

	var done DoneMessage
	json.Unmarshal(doneMsg.Data, &done)
	if done.Error != "idle timeout" {
		t.Errorf("error = %q, want 'idle timeout'", done.Error)
	}
}

func TestHandleShellStart_ConcurrentSessions(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.concurrent", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	const numSessions = 5
	var wg sync.WaitGroup
	wg.Add(numSessions)

	for i := range numSessions {
		go func(idx int) {
			defer wg.Done()

			sessionID := fmt.Sprintf("concurrent-%d", idx)
			req := StartRequest{
				SessionID: sessionID,
				Cols:      80,
				Rows:      24,
			}
			data, _ := json.Marshal(req)

			resp, err := nc.Request("test.shell.concurrent", data, 5*time.Second)
			if err != nil {
				t.Errorf("session %d: request failed: %v", idx, err)
				return
			}

			var startResp StartResponse
			if err := json.Unmarshal(resp.Data, &startResp); err != nil {
				t.Errorf("session %d: unmarshal failed: %v", idx, err)
				return
			}

			if startResp.SessionID != sessionID {
				t.Errorf("session %d: SessionID = %q, want %q", idx, startResp.SessionID, sessionID)
			}

			// Clean up.
			nc.Publish(startResp.InputSubject, []byte("exit\n"))
			nc.Flush()
		}(i)
	}

	wg.Wait()
	time.Sleep(time.Second) // let shells finish
}

func TestSproutSession_Close_Idempotent(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.close", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "close-idempotent",
		Cols:      80,
		Rows:      24,
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.close", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var startResp StartResponse
	json.Unmarshal(resp.Data, &startResp)

	doneSub, err := nc.SubscribeSync(startResp.DoneSubject)
	if err != nil {
		t.Fatalf("subscribe done: %v", err)
	}
	defer doneSub.Unsubscribe()

	// Exit the shell and wait for done.
	nc.Publish(startResp.InputSubject, []byte("exit\n"))
	nc.Flush()

	_, err = doneSub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("timed out waiting for done: %v", err)
	}

	// Sending more input after close should not panic.
	nc.Publish(startResp.InputSubject, []byte("should be ignored\n"))
	nc.Flush()
	time.Sleep(200 * time.Millisecond)
}

func TestHandleShellStart_InvalidShell(t *testing.T) {
	nc := startTestNATS(t)

	sub, err := nc.Subscribe("test.shell.badshell", func(msg *nats.Msg) {
		HandleShellStart(nc, msg)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	req := StartRequest{
		SessionID: "bad-shell-test",
		Cols:      80,
		Rows:      24,
		Shell:     "/nonexistent/shell",
	}
	data, _ := json.Marshal(req)

	resp, err := nc.Request("test.shell.badshell", data, 5*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Data, &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected error for nonexistent shell")
	}
	if !strings.Contains(errResp.Error, "failed to start shell") {
		t.Errorf("error = %q, want to contain 'failed to start shell'", errResp.Error)
	}
}
