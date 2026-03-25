package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// startTestNATS launches an embedded NATS server and returns a connected client.
// The caller should defer nc.Close().
func startTestNATS(t *testing.T) *nats.Conn {
	t.Helper()
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1, // random port
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(ns.Shutdown)

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("connect to NATS: %v", err)
	}
	t.Cleanup(nc.Close)

	return nc
}

func TestCook_RegisteredSproutWithNATS(t *testing.T) {
	dir := setupPKIDirs(t)
	addTestSprout(t, dir, "accepted", "nats-sprout", "UKEY123")

	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "nats-sprout"}},
		Action: map[string]interface{}{"recipe": "test-recipe"},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the response contains a JID
	var cmd apitypes.CmdCook
	if err := json.Unmarshal(w.Body.Bytes(), &cmd); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if cmd.JID == "" {
		t.Error("expected non-empty JID in response")
	}
}

func TestCook_MultipleSproutsWithNATS(t *testing.T) {
	dir := setupPKIDirs(t)
	addTestSprout(t, dir, "accepted", "sprout-a", "UKEYA")
	addTestSprout(t, dir, "accepted", "sprout-b", "UKEYB")

	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{
			{SproutID: "sprout-a"},
			{SproutID: "sprout-b"},
		},
		Action: map[string]interface{}{"recipe": "multi-recipe"},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var cmd apitypes.CmdCook
	json.Unmarshal(w.Body.Bytes(), &cmd)
	if cmd.JID == "" {
		t.Error("expected JID")
	}
}

func TestCancelJob_RunningJobWithNATS(t *testing.T) {
	setupTestJobStore(t)

	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	// jid-002 is a running job (step in progress)
	req := httptest.NewRequest(http.MethodDelete, "/jobs/jid-002", nil)
	w := serveWithPathValues("DELETE /jobs/{jid}", CancelJob, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["jid"] != "jid-002" {
		t.Errorf("expected jid 'jid-002', got %q", resp["jid"])
	}
	if resp["sprout"] != "sprout-alpha" {
		t.Errorf("expected sprout 'sprout-alpha', got %q", resp["sprout"])
	}
	if resp["message"] != "cancel request published" {
		t.Errorf("expected 'cancel request published', got %q", resp["message"])
	}
}

func TestProbeSprout_WithNATS_NoResponder(t *testing.T) {
	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	// No subscriber on the sprout topic, so probeSprout should return false
	// (due to timeout or no responders error)
	result := probeSprout("nonexistent-sprout")
	if result {
		t.Error("expected false for non-responding sprout")
	}
}

func TestProbeSprout_WithNATS_Responder(t *testing.T) {
	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	// Subscribe to the ping topic and respond with a pong
	_, err := nc.Subscribe("grlx.sprouts.alive-sprout.test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Pong: true}
		data, _ := json.Marshal(pong)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	nc.Flush()

	result := probeSprout("alive-sprout")
	if !result {
		t.Error("expected true for responding sprout")
	}
}

func TestProbeSprout_WithNATS_BadResponse(t *testing.T) {
	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	// Subscribe but respond with invalid JSON
	_, err := nc.Subscribe("grlx.sprouts.bad-sprout.test.ping", func(msg *nats.Msg) {
		msg.Respond([]byte("not json"))
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	nc.Flush()

	result := probeSprout("bad-sprout")
	if result {
		t.Error("expected false for bad response")
	}
}

func TestProbeSprout_WithNATS_PongFalse(t *testing.T) {
	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	// Respond with pong=false
	_, err := nc.Subscribe("grlx.sprouts.false-sprout.test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Pong: false}
		data, _ := json.Marshal(pong)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	nc.Flush()

	result := probeSprout("false-sprout")
	if result {
		t.Error("expected false when pong=false")
	}
}

func TestCook_GoroutineWithTrigger(t *testing.T) {
	dir := setupPKIDirs(t)
	addTestSprout(t, dir, "accepted", "triggered-sprout", "UTRIGGERED")

	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "triggered-sprout"}},
		Action: map[string]interface{}{"recipe": "trigger-recipe"},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cmd apitypes.CmdCook
	if err := json.Unmarshal(w.Body.Bytes(), &cmd); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	jid := cmd.JID
	if jid == "" {
		t.Fatal("expected non-empty JID")
	}

	// Publish a request to the cook trigger subject that the goroutine subscribes to.
	// The goroutine will pick it up, reply with sprout IDs, and call SendCookEvent.
	triggerSubject := fmt.Sprintf("grlx.farmer.cook.trigger.%s", jid)
	msg, err := nc.Request(triggerSubject, []byte("trigger"), 5*time.Second)
	if err != nil {
		t.Fatalf("trigger request failed: %v", err)
	}

	// The goroutine should reply with the sprout IDs
	var sproutIDs []string
	if err := json.Unmarshal(msg.Data, &sproutIDs); err != nil {
		t.Fatalf("failed to decode trigger reply: %v", err)
	}
	if len(sproutIDs) != 1 || sproutIDs[0] != "triggered-sprout" {
		t.Errorf("expected [triggered-sprout], got %v", sproutIDs)
	}

	// Give the goroutine time to finish SendCookEvent
	time.Sleep(100 * time.Millisecond)
}

func TestListSprouts_WithNATSAndAcceptedSprout(t *testing.T) {
	tmpDir := setupTestPKI(t)
	addTestSprout(t, tmpDir, "accepted", "online-sprout", "UONLINE1")

	nc := startTestNATS(t)
	oldConn := conn
	conn = nc
	defer func() { conn = oldConn }()

	// No responder, so connected should be false
	req := httptest.NewRequest(http.MethodGet, "/sprouts", nil)
	w := httptest.NewRecorder()

	ListSprouts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp SproutListResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Sprouts) != 1 {
		t.Fatalf("expected 1 sprout, got %d", len(resp.Sprouts))
	}
	// No responder, so connected should be false
	if resp.Sprouts[0].Connected {
		t.Error("expected connected=false without responder")
	}
}
