package cmd

import (
	"encoding/json"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	pkiclient "github.com/gogrlx/grlx/v2/internal/api/client"
	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// startTestNATS starts an embedded NATS server and wires pkiclient.NatsConn.
func startTestNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()
	opts := &natsserver.Options{Host: "127.0.0.1", Port: -1}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("start NATS: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS not ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect NATS: %v", err)
	}
	pkiclient.NatsConn = nc
	oldTimeout := pkiclient.NatsRequestTimeout
	pkiclient.NatsRequestTimeout = 2 * time.Second
	return nc, func() {
		pkiclient.NatsRequestTimeout = oldTimeout
		pkiclient.NatsConn = nil
		nc.Close()
		ns.Shutdown()
	}
}

// natsReply is the envelope that NatsRequest expects.
type natsReply struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// mockPKIList subscribes to grlx.api.pki.list and returns the given accepted sprouts.
func mockPKIList(t *testing.T, nc *nats.Conn, accepted []string) *nats.Subscription {
	t.Helper()
	keys := pki.KeysByType{
		Accepted: pki.KeySet{Sprouts: make([]pki.KeyManager, len(accepted))},
	}
	for i, id := range accepted {
		keys.Accepted.Sprouts[i] = pki.KeyManager{SproutID: id}
	}
	data, _ := json.Marshal(keys)
	reply := natsReply{Result: data}

	sub, err := nc.Subscribe("grlx.api.pki.list", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatalf("subscribe pki.list: %v", err)
	}
	nc.Flush()
	return sub
}

// mockCmdRun subscribes to grlx.api.cmd.run and responds with TargetedResults.
func mockCmdRun(t *testing.T, nc *nats.Conn, results apitypes.TargetedResults) *nats.Subscription {
	t.Helper()
	data, _ := json.Marshal(results)
	reply := natsReply{Result: data}
	sub, err := nc.Subscribe("grlx.api.cmd.run", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatalf("subscribe cmd.run: %v", err)
	}
	nc.Flush()
	return sub
}

func TestFRun_SingleTarget(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{
				"stdout":  "hello\n",
				"errcode": float64(0),
			},
		},
	}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	cmd := apitypes.CmdRun{Command: "echo", Args: []string{"hello"}}
	got, err := FRun("web-01", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	if got.Results == nil {
		t.Fatal("expected non-nil results")
	}
	if _, ok := got.Results["web-01"]; !ok {
		t.Fatal("expected result for web-01")
	}
}

func TestFRun_MultipleTargets(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01", "web-02"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"stdout": "a"},
			"web-02": map[string]interface{}{"stdout": "b"},
		},
	}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	cmd := apitypes.CmdRun{Command: "hostname"}
	got, err := FRun("web-.*", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got.Results))
	}
}

func TestFRun_CommaSeparatedTargets(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01", "db-01", "web-02"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"stdout": "ok"},
			"db-01":  map[string]interface{}{"stdout": "ok"},
		},
	}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	cmd := apitypes.CmdRun{Command: "uptime"}
	got, err := FRun("web-01,db-01", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	if got.Results == nil {
		t.Fatal("expected non-nil results")
	}
}

func TestFRun_NoMatchingTargets(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"db-01"})
	defer sub1.Unsubscribe()

	// The regex won't match anything, so ResolveTargets returns empty.
	// FRun should still succeed with an empty target list.
	want := apitypes.TargetedResults{Results: map[string]interface{}{}}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	cmd := apitypes.CmdRun{Command: "echo"}
	got, err := FRun("web-.*", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	if len(got.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got.Results))
	}
}

func TestFRun_NATSConnNil(t *testing.T) {
	old := pkiclient.NatsConn
	pkiclient.NatsConn = nil
	defer func() { pkiclient.NatsConn = old }()

	cmd := apitypes.CmdRun{Command: "echo"}
	_, err := FRun("web-01", cmd)
	if err == nil {
		t.Fatal("expected error for nil NatsConn")
	}
}

func TestFRun_PKIListError(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	// Mock pki.list to return an error
	reply := natsReply{Error: "permission denied"}
	sub, err := nc.Subscribe("grlx.api.pki.list", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	nc.Flush()

	cmd := apitypes.CmdRun{Command: "echo"}
	_, err = FRun("web-01", cmd)
	if err == nil {
		t.Fatal("expected error from PKI list failure")
	}
}

func TestFRun_CmdRunError(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	// Mock cmd.run to return an error
	reply := natsReply{Error: "command execution failed"}
	sub2, err := nc.Subscribe("grlx.api.cmd.run", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Unsubscribe()
	nc.Flush()

	cmd := apitypes.CmdRun{Command: "bad-cmd"}
	_, err = FRun("web-01", cmd)
	if err == nil {
		t.Fatal("expected error from cmd.run failure")
	}
}

func TestFRun_InvalidJSONResponse(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	// Return a result that's not valid TargetedResults JSON
	reply := natsReply{Result: json.RawMessage(`"just a string"`)}
	sub2, err := nc.Subscribe("grlx.api.cmd.run", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Unsubscribe()
	nc.Flush()

	cmd := apitypes.CmdRun{Command: "echo"}
	_, err = FRun("web-01", cmd)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestFRun_WithTimeout(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"stdout": "done"},
		},
	}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	// CmdRun with a timeout should temporarily adjust NatsRequestTimeout
	cmd := apitypes.CmdRun{
		Command: "sleep",
		Args:    []string{"1"},
		Timeout: 10 * time.Second,
	}

	origTimeout := pkiclient.NatsRequestTimeout
	got, err := FRun("web-01", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	// Verify timeout was restored
	if pkiclient.NatsRequestTimeout != origTimeout {
		t.Fatalf("NatsRequestTimeout not restored: got %v, want %v",
			pkiclient.NatsRequestTimeout, origTimeout)
	}
	if _, ok := got.Results["web-01"]; !ok {
		t.Fatal("expected result for web-01")
	}
}

func TestFRun_NATSTimeout(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	// No handler for cmd.run — will timeout
	oldTimeout := pkiclient.NatsRequestTimeout
	pkiclient.NatsRequestTimeout = 100 * time.Millisecond
	defer func() { pkiclient.NatsRequestTimeout = oldTimeout }()

	cmd := apitypes.CmdRun{Command: "echo"}
	_, err := FRun("web-01", cmd)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFRun_RegexTarget(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01", "web-02", "db-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"stdout": "a"},
			"web-02": map[string]interface{}{"stdout": "b"},
		},
	}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	cmd := apitypes.CmdRun{Command: "hostname"}
	got, err := FRun("web-.*", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got.Results))
	}
}

func TestFRun_ErrorCodePreserved(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{
				"command": "false",
				"stderr":  "failed",
				"errcode": float64(1),
			},
		},
	}
	sub2 := mockCmdRun(t, nc, want)
	defer sub2.Unsubscribe()

	cmd := apitypes.CmdRun{Command: "false"}
	got, err := FRun("web-01", cmd)
	if err != nil {
		t.Fatalf("FRun: %v", err)
	}
	result, ok := got.Results["web-01"]
	if !ok {
		t.Fatal("expected result for web-01")
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if errCode, ok := m["errcode"].(float64); !ok || int(errCode) != 1 {
		t.Fatalf("expected errcode=1, got %v", m["errcode"])
	}
}
