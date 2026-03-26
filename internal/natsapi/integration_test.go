package natsapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/pki"
	"github.com/gogrlx/grlx/v2/internal/rbac"
	"github.com/gogrlx/grlx/v2/internal/shell"
)

// startEmbeddedNATS starts an embedded NATS server for integration tests.
// Returns the nats.Conn and a cleanup function.
func startEmbeddedNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
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

// --- Subscribe integration tests ---

func TestSubscribeRegistersAllRoutes(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	defer func() { natsConn = old }()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Verify we can send a request to a registered route and get a response.
	// Use the version endpoint since it has no external dependencies.
	SetBuildVersion(config.Version{Tag: "v1.0.0-test"})
	defer SetBuildVersion(config.Version{})

	msg, err := nc.Request("grlx.api.version", nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request to grlx.api.version: %v", err)
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
	if ver.Tag != "v1.0.0-test" {
		t.Errorf("Tag = %q, want %q", ver.Tag, "v1.0.0-test")
	}
}

func TestSubscribeTestPingRoute(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	defer func() { natsConn = old }()

	setupNatsAPIPKI(t)
	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// test.ping with empty targets — should succeed.
	params, _ := json.Marshal(apitypes.TargetedAction{
		Target: []pki.KeyManager{},
		Action: apitypes.PingPong{Ping: true},
	})

	msg, err := nc.Request("grlx.api.test.ping", params, 2*time.Second)
	if err != nil {
		t.Fatalf("request to grlx.api.test.ping: %v", err)
	}

	var resp response
	json.Unmarshal(msg.Data, &resp)
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

func TestSubscribeJobsListRoute(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	defer func() { natsConn = old }()

	_, jobCleanup := setupJobStore(t)
	defer jobCleanup()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	msg, err := nc.Request("grlx.api.jobs.list", nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request to grlx.api.jobs.list: %v", err)
	}

	var resp response
	json.Unmarshal(msg.Data, &resp)
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

func TestSubscribePropsSetGetRoute(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Set a prop via NATS.
	setParams, _ := json.Marshal(PropsParams{
		SproutID: "integration-sprout",
		Name:     "env",
		Value:    "testing",
	})
	msg, err := nc.Request("grlx.api.props.set", setParams, 2*time.Second)
	if err != nil {
		t.Fatalf("props.set request: %v", err)
	}
	var setResp response
	json.Unmarshal(msg.Data, &setResp)
	if setResp.Error != "" {
		t.Fatalf("props.set error: %s", setResp.Error)
	}

	// Get it back.
	getParams, _ := json.Marshal(PropsParams{
		SproutID: "integration-sprout",
		Name:     "env",
	})
	msg, err = nc.Request("grlx.api.props.get", getParams, 2*time.Second)
	if err != nil {
		t.Fatalf("props.get request: %v", err)
	}
	var getResp response
	json.Unmarshal(msg.Data, &getResp)
	if getResp.Error != "" {
		t.Fatalf("props.get error: %s", getResp.Error)
	}

	b, _ := json.Marshal(getResp.Result)
	var m map[string]string
	json.Unmarshal(b, &m)
	if m["value"] != "testing" {
		t.Errorf("value = %q, want %q", m["value"], "testing")
	}
}

func TestSubscribeHandlerError(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	defer func() { natsConn = old }()

	_, jobCleanup := setupJobStore(t)
	defer jobCleanup()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Request a nonexistent job — should return error in response envelope.
	params, _ := json.Marshal(JobsGetParams{JID: "nonexistent-jid"})
	msg, err := nc.Request("grlx.api.jobs.get", params, 2*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var resp response
	json.Unmarshal(msg.Data, &resp)
	if resp.Error == "" {
		t.Fatal("expected error in response for nonexistent job")
	}
}

func TestSubscribeWithAuditLogging(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	defer func() { natsConn = old }()

	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	if err := Subscribe(nc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Version request should be logged.
	SetBuildVersion(config.Version{Tag: "audit-test"})
	defer SetBuildVersion(config.Version{})

	msg, err := nc.Request("grlx.api.version", nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var resp response
	json.Unmarshal(msg.Data, &resp)
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

// --- probeSprout integration test ---

func TestProbeSproutSuccess(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Subscribe a mock sprout that responds to ping.
	_, err := nc.Subscribe("grlx.sprouts.test-sprout.test.ping", func(msg *nats.Msg) {
		resp, _ := json.Marshal(apitypes.PingPong{Pong: true})
		msg.Respond(resp)
	})
	if err != nil {
		t.Fatalf("subscribe mock sprout: %v", err)
	}
	nc.Flush()

	if !probeSprout("test-sprout") {
		t.Error("expected probeSprout to return true for responding sprout")
	}
}

func TestProbeSproutTimeout(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// No subscriber for this sprout — should timeout and return false.
	if probeSprout("nonexistent-sprout") {
		t.Error("expected probeSprout to return false for unresponsive sprout")
	}
}

func TestProbeSproutBadResponse(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Subscribe a mock sprout that returns invalid JSON.
	nc.Subscribe("grlx.sprouts.bad-json-sprout.test.ping", func(msg *nats.Msg) {
		msg.Respond([]byte(`{invalid json`))
	})
	nc.Flush()

	if probeSprout("bad-json-sprout") {
		t.Error("expected probeSprout to return false for bad JSON response")
	}
}

func TestProbeSproutNoPong(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Responds with valid JSON but Pong=false.
	nc.Subscribe("grlx.sprouts.no-pong.test.ping", func(msg *nats.Msg) {
		resp, _ := json.Marshal(apitypes.PingPong{Pong: false})
		msg.Respond(resp)
	})
	nc.Flush()

	if probeSprout("no-pong") {
		t.Error("expected probeSprout to return false when Pong is false")
	}
}

// --- handleCook integration test ---

func TestHandleCookSuccessWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-cook-int", "UKEY_COOK_INT")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	params, _ := json.Marshal(map[string]interface{}{
		"target": []map[string]string{{"id": "sprout-cook-int"}},
		"action": map[string]string{"recipe": "test.recipe"},
	})

	result, err := handleCook(params)
	if err != nil {
		t.Fatalf("handleCook: %v", err)
	}

	// Should return a CmdCook with a generated JID.
	cmd, ok := result.(apitypes.CmdCook)
	if !ok {
		t.Fatalf("result type = %T, want apitypes.CmdCook", result)
	}
	if cmd.JID == "" {
		t.Error("expected non-empty JID")
	}
	if cmd.Recipe != "test.recipe" {
		t.Errorf("Recipe = %q, want %q", cmd.Recipe, "test.recipe")
	}
}

func TestHandleCookWithTokenInvoker(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-cook-tk", "UKEY_COOK_TK")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	token, authCleanup := setupAuthWithToken(t, "operator", []rbac.Rule{
		{Action: rbac.ActionCook, Scope: "*"},
	})
	defer authCleanup()

	params, _ := json.Marshal(map[string]interface{}{
		"token":  token,
		"target": []map[string]string{{"id": "sprout-cook-tk"}},
		"action": map[string]string{"recipe": "deploy.recipe"},
	})

	result, err := handleCook(params)
	if err != nil {
		t.Fatalf("handleCook: %v", err)
	}

	cmd := result.(apitypes.CmdCook)
	if cmd.JID == "" {
		t.Error("expected non-empty JID")
	}
}

func TestHandleCookMultipleTargets(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-multi-1", "UKEY_M1")
	writeNKey(t, pkiDir, "accepted", "sprout-multi-2", "UKEY_M2")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	params, _ := json.Marshal(map[string]interface{}{
		"target": []map[string]string{
			{"id": "sprout-multi-1"},
			{"id": "sprout-multi-2"},
		},
		"action": map[string]string{"recipe": "multi.recipe"},
	})

	result, err := handleCook(params)
	if err != nil {
		t.Fatalf("handleCook: %v", err)
	}

	cmd := result.(apitypes.CmdCook)
	if cmd.JID == "" {
		t.Error("expected non-empty JID")
	}
}

func TestHandleCookTestMode(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-test-mode", "UKEY_TM")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	params, _ := json.Marshal(map[string]interface{}{
		"target": []map[string]string{{"id": "sprout-test-mode"}},
		"action": map[string]interface{}{"recipe": "dry-run.recipe", "test": true},
	})

	result, err := handleCook(params)
	if err != nil {
		t.Fatalf("handleCook: %v", err)
	}

	cmd := result.(apitypes.CmdCook)
	if cmd.JID == "" {
		t.Error("expected non-empty JID")
	}
}

func TestHandleCookUnregisteredSprout(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	setupNatsAPIPKI(t)

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	params, _ := json.Marshal(map[string]interface{}{
		"target": []map[string]string{{"id": "unregistered-sprout"}},
		"action": map[string]string{"recipe": "test.recipe"},
	})

	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error for unregistered sprout")
	}
}

func TestHandleCookInvalidJSONWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	_, err := handleCook(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- handleShellStart integration test ---

func TestHandleShellStartSuccess(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-shell-int", "UKEY_SHELL_INT")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Subscribe a mock sprout that responds to shell.start.
	nc.Subscribe("grlx.sprouts.sprout-shell-int.shell.start", func(msg *nats.Msg) {
		var req shell.StartRequest
		json.Unmarshal(msg.Data, &req)

		resp := shell.StartResponse{
			SessionID:     req.SessionID,
			InputSubject:  "grlx.shell." + req.SessionID + ".input",
			OutputSubject: "grlx.shell." + req.SessionID + ".output",
			DoneSubject:   "grlx.shell." + req.SessionID + ".done",
		}
		data, _ := json.Marshal(resp)
		msg.Respond(data)
	})
	nc.Flush()

	params, _ := json.Marshal(shell.CLIStartRequest{
		SproutID: "sprout-shell-int",
		Cols:     80,
		Rows:     24,
	})

	result, err := handleShellStart(params)
	if err != nil {
		t.Fatalf("handleShellStart: %v", err)
	}

	resp, ok := result.(shell.StartResponse)
	if !ok {
		t.Fatalf("result type = %T, want shell.StartResponse", result)
	}
	if resp.SessionID == "" {
		t.Error("expected non-empty SessionID")
	}
	if resp.InputSubject == "" {
		t.Error("expected non-empty InputSubject")
	}
	if resp.DoneSubject == "" {
		t.Error("expected non-empty DoneSubject")
	}

	// Verify session was tracked.
	tracker := ShellTracker()
	sessions := tracker.List()
	found := false
	for _, s := range sessions {
		if s.SproutID == "sprout-shell-int" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session to be tracked")
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

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Mock sprout that returns an error.
	nc.Subscribe("grlx.sprouts.sprout-shell-err.shell.start", func(msg *nats.Msg) {
		resp, _ := json.Marshal(map[string]string{"error": "shell not available"})
		msg.Respond(resp)
	})
	nc.Flush()

	params, _ := json.Marshal(shell.CLIStartRequest{
		SproutID: "sprout-shell-err",
		Cols:     80,
		Rows:     24,
	})

	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error when sprout returns error")
	}
}

func TestHandleShellStartEmptySproutID(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	params, _ := json.Marshal(shell.CLIStartRequest{
		SproutID: "",
		Cols:     80,
		Rows:     24,
	})

	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error for empty sprout_id")
	}
}

func TestHandleShellStartInvalidJSONWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	_, err := handleShellStart(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleShellStartTimeout(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-shell-timeout", "UKEY_SHELL_TO")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// No subscriber — will timeout.
	params, _ := json.Marshal(shell.CLIStartRequest{
		SproutID: "sprout-shell-timeout",
		Cols:     80,
		Rows:     24,
	})

	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error for sprout timeout")
	}
}

// --- subscribeSessionDone integration test ---

func TestSubscribeSessionDoneReceivesMessage(t *testing.T) {
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
		SessionID:   "int-sess-done",
		SproutID:    "sprout-done-test",
		Pubkey:      "UTESTDONE",
		RoleName:    "admin",
		Shell:       "/bin/bash",
		StartedAt:   time.Now().Add(-2 * time.Minute),
		DoneSubject: "grlx.shell.int-sess-done.done",
	}
	sessionTracker.Add(info)

	subscribeSessionDone(info)

	// Publish done message.
	doneMsg := shell.DoneMessage{
		ExitCode: 0,
	}
	data, _ := json.Marshal(doneMsg)
	if err := nc.Publish(info.DoneSubject, data); err != nil {
		t.Fatalf("publish done: %v", err)
	}
	nc.Flush()

	// Wait for the subscription to process.
	time.Sleep(100 * time.Millisecond)

	// Session should be removed from tracker.
	if s := sessionTracker.Get("int-sess-done"); s != nil {
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
		SessionID:   "int-sess-err",
		SproutID:    "sprout-err-test",
		Pubkey:      "UTESTERR",
		RoleName:    "operator",
		StartedAt:   time.Now().Add(-1 * time.Minute),
		DoneSubject: "grlx.shell.int-sess-err.done",
	}
	sessionTracker.Add(info)

	subscribeSessionDone(info)

	doneMsg := shell.DoneMessage{
		ExitCode: 1,
		Error:    "connection lost",
	}
	data, _ := json.Marshal(doneMsg)
	nc.Publish(info.DoneSubject, data)
	nc.Flush()

	time.Sleep(100 * time.Millisecond)

	if s := sessionTracker.Get("int-sess-err"); s != nil {
		t.Error("expected session to be removed from tracker")
	}
}

// --- handleSproutsList with NATS (probeSprout path) ---

func TestHandleSproutsListWithConnectedSprout(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-connected", "UKEY_CONN")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Subscribe mock sprout to respond to ping.
	nc.Subscribe("grlx.sprouts.sprout-connected.test.ping", func(msg *nats.Msg) {
		resp, _ := json.Marshal(apitypes.PingPong{Pong: true})
		msg.Respond(resp)
	})
	nc.Flush()

	result, err := handleSproutsList(nil)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m := result.(map[string][]SproutInfo)
	found := false
	for _, s := range m["sprouts"] {
		if s.ID == "sprout-connected" {
			found = true
			if !s.Connected {
				t.Error("expected Connected=true for responding sprout")
			}
			break
		}
	}
	if !found {
		t.Error("sprout-connected not in list")
	}
}

func TestHandleSproutsGetWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-get-int", "UKEY_GET_INT")

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	// Mock sprout responds to ping.
	nc.Subscribe("grlx.sprouts.sprout-get-int.test.ping", func(msg *nats.Msg) {
		resp, _ := json.Marshal(apitypes.PingPong{Pong: true})
		msg.Respond(resp)
	})
	nc.Flush()

	params, _ := json.Marshal(pki.KeyManager{SproutID: "sprout-get-int"})
	result, err := handleSproutsGet(params)
	if err != nil {
		t.Fatalf("handleSproutsGet: %v", err)
	}

	info := result.(SproutInfo)
	if info.ID != "sprout-get-int" {
		t.Errorf("ID = %q, want %q", info.ID, "sprout-get-int")
	}
	if !info.Connected {
		t.Error("expected Connected=true")
	}
	if info.KeyState != "accepted" {
		t.Errorf("KeyState = %q, want %q", info.KeyState, "accepted")
	}
}

// --- handleJobsCancel with NATS ---

func TestHandleJobsCancelWithNATS(t *testing.T) {
	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	dir, jobCleanup := setupJobStore(t)
	defer jobCleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	// Create a running job (no completed steps = running status).
	steps := []cook.StepCompletion{
		{ID: "s1", Started: time.Now()},
	}
	writeTestJob(t, dir, "sprout-cancel-int", "jid-cancel-int", steps)

	// Subscribe to capture the cancel message.
	cancelReceived := make(chan bool, 1)
	nc.Subscribe("grlx.sprouts.sprout-cancel-int.cancel", func(msg *nats.Msg) {
		cancelReceived <- true
	})
	nc.Flush()

	params, _ := json.Marshal(JobsGetParams{JID: "jid-cancel-int"})
	result, err := handleJobsCancel(params)
	if err != nil {
		t.Fatalf("handleJobsCancel: %v", err)
	}

	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("result type = %T, want map[string]string", result)
	}
	if m["jid"] != "jid-cancel-int" {
		t.Errorf("jid = %q, want %q", m["jid"], "jid-cancel-int")
	}

	// Verify cancel was published.
	select {
	case <-cancelReceived:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("cancel message not received within timeout")
	}
}
