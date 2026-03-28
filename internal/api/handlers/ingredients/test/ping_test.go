package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/config"
	ingredtest "github.com/gogrlx/grlx/v2/internal/ingredients/test"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func setupPingTestPKI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.FarmerPKI = dir + "/"
	for _, state := range []string{"accepted", "unaccepted", "denied", "rejected"} {
		if err := os.MkdirAll(filepath.Join(dir, "sprouts", state), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func addPingTestSprout(t *testing.T, dir, state, id, nkey string) {
	t.Helper()
	path := filepath.Join(dir, "sprouts", state, id)
	if err := os.WriteFile(path, []byte(nkey), 0o644); err != nil {
		t.Fatal(err)
	}
}

// startTestNATSServer starts an embedded NATS server and registers the
// connection with the ingredients/test package. Returns a cleanup func.
func startTestNATSServer(t *testing.T) (*nats.Conn, func()) {
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

	ingredtest.RegisterNatsConn(nc)

	return nc, func() {
		nc.Close()
		ns.Shutdown()
	}
}

func TestHTestPing_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTestPing_InvalidSproutID(t *testing.T) {
	setupPingTestPKI(t)

	body := []byte(`{"target":[{"id":"INVALID-CAPS"}],"action":{"ping":true}}`)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTestPing_SproutIDWithUnderscore(t *testing.T) {
	setupPingTestPKI(t)

	body := []byte(`{"target":[{"id":"bad_id"}],"action":{"ping":true}}`)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTestPing_UnknownSprout(t *testing.T) {
	setupPingTestPKI(t)

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "unknown-sprout"}},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHTestPing_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader([]byte("")))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestHTestPing_MultipleTargetsOneInvalid(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "valid-sprout", "UKEY1")

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{
			{SproutID: "valid-sprout"},
			{SproutID: "INVALID"},
		},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTestPing_MultipleTargetsOneUnregistered(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "real-sprout", "UKEY1")

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{
			{SproutID: "real-sprout"},
			{SproutID: "ghost-sprout"},
		},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// === Happy path tests (require embedded NATS) ===

func TestHTestPing_SingleSproutSuccess(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "sprout1", "UKEY1")

	nc, cleanup := startTestNATSServer(t)
	defer cleanup()

	// Mock the sprout responding to ping
	sub, err := nc.Subscribe("grlx.sprouts.sprout1.test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Ping: false, Pong: true}
		data, _ := json.Marshal(pong)
		_ = msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe mock: %v", err)
	}
	defer sub.Unsubscribe()

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "sprout1"}},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var results apitypes.TargetedResults
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if results.Results == nil {
		t.Fatal("expected non-nil results map")
	}
	if _, ok := results.Results["sprout1"]; !ok {
		t.Fatal("expected results to contain sprout1")
	}
}

func TestHTestPing_MultipleSproutsSuccess(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "sprout-a", "UKEYA")
	addPingTestSprout(t, dir, "accepted", "sprout-b", "UKEYB")

	nc, cleanup := startTestNATSServer(t)
	defer cleanup()

	// Mock both sprouts responding
	for _, id := range []string{"sprout-a", "sprout-b"} {
		sproutID := id
		sub, err := nc.Subscribe("grlx.sprouts."+sproutID+".test.ping", func(msg *nats.Msg) {
			pong := apitypes.PingPong{Ping: false, Pong: true}
			data, _ := json.Marshal(pong)
			_ = msg.Respond(data)
		})
		if err != nil {
			t.Fatalf("subscribe mock for %s: %v", sproutID, err)
		}
		defer sub.Unsubscribe()
	}

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{
			{SproutID: "sprout-a"},
			{SproutID: "sprout-b"},
		},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var results apitypes.TargetedResults
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(results.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results.Results))
	}
	for _, id := range []string{"sprout-a", "sprout-b"} {
		if _, ok := results.Results[id]; !ok {
			t.Fatalf("expected results to contain %s", id)
		}
	}
}

func TestHTestPing_SproutTimeout(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "slow-sprout", "UKEY1")

	_, cleanup := startTestNATSServer(t)
	defer cleanup()

	// No subscriber — the ping will time out via NATS Request timeout.
	// FPing uses 15s timeout, but we're testing that the handler still
	// returns 200 with error info rather than crashing.
	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "slow-sprout"}},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	// Handler still returns 200 — the error is in the result value
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even on timeout, got %d", w.Code)
	}

	var results apitypes.TargetedResults
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := results.Results["slow-sprout"]; !ok {
		t.Fatal("expected results to contain slow-sprout (even with error)")
	}
}

func TestHTestPing_EmptyTargetList(t *testing.T) {
	setupPingTestPKI(t)

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	// Empty target list — no validation failures, goes straight to fan-out
	// with zero goroutines, returns 200 with empty/nil results.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTestPing_SproutInvalidResponseJSON(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "bad-json-sprout", "UKEY1")

	nc, cleanup := startTestNATSServer(t)
	defer cleanup()

	// Mock sprout returns invalid JSON
	sub, err := nc.Subscribe("grlx.sprouts.bad-json-sprout.test.ping", func(msg *nats.Msg) {
		_ = msg.Respond([]byte("not valid json"))
	})
	if err != nil {
		t.Fatalf("subscribe mock: %v", err)
	}
	defer sub.Unsubscribe()

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "bad-json-sprout"}},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	// Handler still returns 200 — errors are per-sprout in the results map
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTestPing_NilAction(t *testing.T) {
	dir := setupPingTestPKI(t)
	addPingTestSprout(t, dir, "accepted", "sprout-nil", "UKEY1")

	nc, cleanup := startTestNATSServer(t)
	defer cleanup()

	sub, err := nc.Subscribe("grlx.sprouts.sprout-nil.test.ping", func(msg *nats.Msg) {
		pong := apitypes.PingPong{Ping: false, Pong: true}
		data, _ := json.Marshal(pong)
		_ = msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("subscribe mock: %v", err)
	}
	defer sub.Unsubscribe()

	// Send with nil action — json.Marshal of PingPong from nil produces zeroes
	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "sprout-nil"}},
		Action: nil,
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTestPing_SproutIDStartsWithDash(t *testing.T) {
	setupPingTestPKI(t)

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "-invalid-start"}},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTestPing_SproutIDEndsWithDot(t *testing.T) {
	setupPingTestPKI(t)

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "invalid."}},
		Action: map[string]interface{}{"ping": true},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/test/ping", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HTestPing(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
