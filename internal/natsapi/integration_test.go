package natsapi

import (
	"encoding/json"
	"testing"
	"time"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/shell"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// startEmbeddedNATS starts an in-process NATS server and returns a connected
// client plus a cleanup function.
func startEmbeddedNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // random port
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start test NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to become ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect to test NATS: %v", err)
	}

	return nc, func() {
		nc.Close()
		ns.Shutdown()
	}
}

// --- Subscribe integration test ---

func TestSubscribeWithRealNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	// Subscribe registers all routes on the connection.
	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Verify we can send a request and get a response (version is stateless).
	want := "v0.0.0-test"
	SetBuildVersion(config.Version{Tag: want})
	defer SetBuildVersion(config.Version{})

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	msg, err := nc.Request("grlx.api.version", nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request grlx.api.version: %v", err)
	}

	var resp response
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	b, _ := json.Marshal(resp.Result)
	var ver config.Version
	json.Unmarshal(b, &ver)
	if ver.Tag != want {
		t.Errorf("version tag = %q, want %q", ver.Tag, want)
	}
}

func TestSubscribeHandlerError(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Send invalid params to a handler that validates input.
	msg, err := nc.Request("grlx.api.jobs.get", json.RawMessage(`{invalid`), 2*time.Second)
	if err != nil {
		t.Fatalf("request grlx.api.jobs.get: %v", err)
	}

	var resp response
	json.Unmarshal(msg.Data, &resp)
	if resp.Error == "" {
		t.Fatal("expected error response for invalid params")
	}
}

func TestSubscribeAuditLogging(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	// Set up audit logging to capture the action.
	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	// Configure audit level to log everything.
	audit.SetLevel(audit.LevelAll)
	defer audit.SetLevel(audit.LevelWrite)

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Make a request — should trigger audit log.
	msg, err := nc.Request("grlx.api.version", nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var resp response
	json.Unmarshal(msg.Data, &resp)
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

// --- probeSprout with real NATS ---

func TestProbeSproutConnected(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	sproutID := "sprout-ping-test"

	// Mock the sprout responding to ping.
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Pong: true}
		data, _ := json.Marshal(pong)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	if !probeSprout(sproutID) {
		t.Error("expected probeSprout to return true for connected sprout")
	}
}

func TestProbeSproutDisconnected(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// No responder → should timeout and return false.
	if probeSprout("nonexistent-sprout") {
		t.Error("expected probeSprout to return false for disconnected sprout")
	}
}

func TestProbeSproutBadResponse(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	sproutID := "sprout-bad-pong"

	// Mock the sprout returning invalid JSON.
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(msg *nats.Msg) {
		msg.Respond([]byte("not-json"))
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	if probeSprout(sproutID) {
		t.Error("expected false for invalid pong response")
	}
}

func TestProbeSproutPongFalse(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	sproutID := "sprout-pong-false"

	// Mock the sprout responding with Pong=false.
	sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Pong: false}
		data, _ := json.Marshal(pong)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	if probeSprout(sproutID) {
		t.Error("expected false when sprout returns Pong=false")
	}
}

// --- handleCook with real NATS ---

func TestHandleCookWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-cook-nats", "UKEY_COOK_NATS")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	params := json.RawMessage(`{"target":[{"id":"sprout-cook-nats"}],"action":{"recipe":"webserver.nginx"}}`)
	result, err := handleCook(params)
	if err != nil {
		t.Fatalf("handleCook: %v", err)
	}

	// Should return a CmdCook with a JID.
	b, _ := json.Marshal(result)
	var cmd struct {
		JID    string `json:"jid"`
		Recipe string `json:"recipe"`
	}
	if err := json.Unmarshal(b, &cmd); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if cmd.JID == "" {
		t.Error("expected non-empty JID")
	}
}

func TestHandleCookInvalidJSONIntegration(t *testing.T) {
	_, err := handleCook(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleCookUnknownSproutIntegration(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"unknown-sprout"}],"action":{"recipe":"test"}}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error for unknown sprout")
	}
}

// --- handleCook goroutine path: trigger + send cook events ---

func TestHandleCookTriggerAndSendEvents(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-cook-trigger", "UKEY_COOK_TRIGGER")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	params := json.RawMessage(`{"target":[{"id":"sprout-cook-trigger"}],"action":{"recipe":"deploy.app"}}`)
	result, err := handleCook(params)
	if err != nil {
		t.Fatalf("handleCook: %v", err)
	}

	b, _ := json.Marshal(result)
	var cmd struct {
		JID string `json:"jid"`
	}
	json.Unmarshal(b, &cmd)

	// Simulate the trigger message arriving (normally from the farmer's cook subsystem).
	triggerSubject := "grlx.farmer.cook.trigger." + cmd.JID
	msg, err := nc.Request(triggerSubject, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("trigger request: %v", err)
	}

	// The goroutine should respond with the sprout IDs.
	var sproutIDs []string
	if err := json.Unmarshal(msg.Data, &sproutIDs); err != nil {
		t.Fatalf("unmarshal trigger response: %v", err)
	}
	if len(sproutIDs) != 1 || sproutIDs[0] != "sprout-cook-trigger" {
		t.Errorf("trigger response = %v, want [sprout-cook-trigger]", sproutIDs)
	}
}

// --- subscribeSessionDone with real NATS ---

func TestSubscribeSessionDoneWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Set up audit logger.
	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	info := &shell.SessionInfo{
		SessionID:   "test-done-nats",
		SproutID:    "test-sprout",
		Pubkey:      "UTESTKEY",
		RoleName:    "admin",
		Shell:       "/bin/bash",
		StartedAt:   time.Now().Add(-2 * time.Minute),
		DoneSubject: "grlx.shell.test-done-nats.done",
	}
	sessionTracker.Add(info)

	subscribeSessionDone(info)

	// Publish a done message.
	done := shell.DoneMessage{ExitCode: 0}
	data, _ := json.Marshal(done)
	if err := nc.Publish(info.DoneSubject, data); err != nil {
		t.Fatalf("publish done: %v", err)
	}
	nc.Flush()

	// Give the callback time to fire.
	time.Sleep(200 * time.Millisecond)

	// Session should be removed from tracker.
	if tracked := sessionTracker.Remove(info.SessionID); tracked != nil {
		t.Error("expected session to be removed from tracker after done")
	}
}

func TestSubscribeSessionDoneWithError(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	info := &shell.SessionInfo{
		SessionID:   "test-done-err",
		SproutID:    "test-sprout",
		Pubkey:      "UTESTKEY",
		RoleName:    "operator",
		StartedAt:   time.Now().Add(-1 * time.Minute),
		DoneSubject: "grlx.shell.test-done-err.done",
	}
	sessionTracker.Add(info)

	subscribeSessionDone(info)

	// Publish a done message with an error.
	done := shell.DoneMessage{ExitCode: 1, Error: "connection lost"}
	data, _ := json.Marshal(done)
	nc.Publish(info.DoneSubject, data)
	nc.Flush()
	time.Sleep(200 * time.Millisecond)

	if tracked := sessionTracker.Remove(info.SessionID); tracked != nil {
		t.Error("expected session removed after error done")
	}
}

func TestSubscribeSessionDoneUntrackedSession(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	info := &shell.SessionInfo{
		SessionID:   "test-done-untracked",
		SproutID:    "test-sprout",
		DoneSubject: "grlx.shell.test-done-untracked.done",
	}
	// Do NOT add to tracker — simulates an already-removed session.

	subscribeSessionDone(info)

	// Publish done — the callback should handle the nil tracker result gracefully.
	done := shell.DoneMessage{ExitCode: 0}
	data, _ := json.Marshal(done)
	nc.Publish(info.DoneSubject, data)
	nc.Flush()
	time.Sleep(200 * time.Millisecond)
	// No panic = pass.
}

// --- handleShellStart with real NATS (sprout responding) ---

func TestHandleShellStartWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-shell-nats", "UKEY_SHELL_NATS")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Mock the sprout responding to shell.start.
	sub, err := nc.Subscribe("grlx.sprouts.sprout-shell-nats.shell.start", func(msg *nats.Msg) {
		var req shell.StartRequest
		json.Unmarshal(msg.Data, &req)

		resp := shell.StartResponse{
			SessionID:     req.SessionID,
			InputSubject:  "grlx.shell." + req.SessionID + ".input",
			OutputSubject: "grlx.shell." + req.SessionID + ".output",
			ResizeSubject: "grlx.shell." + req.SessionID + ".resize",
			DoneSubject:   "grlx.shell." + req.SessionID + ".done",
		}
		data, _ := json.Marshal(resp)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	params := json.RawMessage(`{"sprout_id":"sprout-shell-nats","cols":80,"rows":24,"shell":"/bin/bash"}`)
	result, err := handleShellStart(params)
	if err != nil {
		t.Fatalf("handleShellStart: %v", err)
	}

	resp, ok := result.(shell.StartResponse)
	if !ok {
		t.Fatalf("result type = %T, want shell.StartResponse", result)
	}
	if resp.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
	if resp.InputSubject == "" || resp.OutputSubject == "" {
		t.Error("expected non-empty subjects in response")
	}
}

func TestHandleShellStartSproutError(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-shell-err", "UKEY_SHELL_ERR")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Mock the sprout returning an error.
	sub, err := nc.Subscribe("grlx.sprouts.sprout-shell-err.shell.start", func(msg *nats.Msg) {
		errResp := map[string]string{"error": "shell not available"}
		data, _ := json.Marshal(errResp)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	params := json.RawMessage(`{"sprout_id":"sprout-shell-err","cols":80,"rows":24}`)
	_, err = handleShellStart(params)
	if err == nil {
		t.Fatal("expected error when sprout returns error")
	}
}

func TestHandleShellStartInvalidJSONIntegration(t *testing.T) {
	_, err := handleShellStart(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleShellStartEmptySproutIDIntegration(t *testing.T) {
	_, err := handleShellStart(json.RawMessage(`{"sprout_id":"","cols":80,"rows":24}`))
	if err == nil {
		t.Fatal("expected error for empty sprout_id")
	}
}

func TestHandleShellStartSproutIDWithUnderscoreIntegration(t *testing.T) {
	setupNatsAPIPKI(t)

	_, err := handleShellStart(json.RawMessage(`{"sprout_id":"sprout_bad","cols":80,"rows":24}`))
	if err == nil {
		t.Fatal("expected error for sprout ID with underscore")
	}
}

// --- handleSproutsGet with real NATS (probe path) ---

func TestHandleSproutsGetWithConnectedSprout(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-get-conn", "UKEY_GET_CONN")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Mock the ping response.
	sub, err := nc.Subscribe("grlx.sprouts.sprout-get-conn.test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Pong: true}
		data, _ := json.Marshal(pong)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	params := json.RawMessage(`{"id":"sprout-get-conn"}`)
	result, err := handleSproutsGet(params)
	if err != nil {
		t.Fatalf("handleSproutsGet: %v", err)
	}

	info, ok := result.(SproutInfo)
	if !ok {
		t.Fatalf("result type = %T, want SproutInfo", result)
	}
	if !info.Connected {
		t.Error("expected Connected=true for responding sprout")
	}
	if info.KeyState != "accepted" {
		t.Errorf("KeyState = %q, want accepted", info.KeyState)
	}
}

// --- handleSproutsList with real NATS (probe path) ---

func TestHandleSproutsListWithConnectedSprouts(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-list-a", "UKEY_LIST_A")
	writeNKey(t, pkiDir, "accepted", "sprout-list-b", "UKEY_LIST_B")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Mock ping for sprout-list-a only.
	sub, err := nc.Subscribe("grlx.sprouts.sprout-list-a.test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Pong: true}
		data, _ := json.Marshal(pong)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	result, err := handleSproutsList(nil)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m := result.(map[string][]SproutInfo)
	sprouts := m["sprouts"]
	if len(sprouts) < 2 {
		t.Fatalf("expected at least 2 sprouts, got %d", len(sprouts))
	}

	connCount := 0
	for _, s := range sprouts {
		if s.Connected {
			connCount++
		}
	}
	if connCount != 1 {
		t.Errorf("expected 1 connected sprout, got %d", connCount)
	}
}

// --- ShellTracker returns the tracker ---

func TestShellTrackerIntegration(t *testing.T) {
	tracker := ShellTracker()
	if tracker == nil {
		t.Fatal("ShellTracker returned nil")
	}
	// Should be the same instance as sessionTracker.
	if tracker != sessionTracker {
		t.Error("ShellTracker returned different instance than sessionTracker")
	}
}
