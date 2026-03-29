package test

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

// mockPKIList subscribes to grlx.api.pki.list and returns accepted sprouts.
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

// mockTestPing subscribes to grlx.api.test.ping and responds with TargetedResults.
func mockTestPing(t *testing.T, nc *nats.Conn, results apitypes.TargetedResults) *nats.Subscription {
	t.Helper()
	data, _ := json.Marshal(results)
	reply := natsReply{Result: data}
	sub, err := nc.Subscribe("grlx.api.test.ping", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatalf("subscribe test.ping: %v", err)
	}
	nc.Flush()
	return sub
}

func TestFPing_SingleTarget(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"pong": true},
		},
	}
	sub2 := mockTestPing(t, nc, want)
	defer sub2.Unsubscribe()

	got, err := FPing("web-01")
	if err != nil {
		t.Fatalf("FPing: %v", err)
	}
	if got.Results == nil {
		t.Fatal("expected non-nil results")
	}
	if _, ok := got.Results["web-01"]; !ok {
		t.Fatal("expected result for web-01")
	}
}

func TestFPing_MultipleTargets(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01", "web-02"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"pong": true},
			"web-02": map[string]interface{}{"pong": true},
		},
	}
	sub2 := mockTestPing(t, nc, want)
	defer sub2.Unsubscribe()

	got, err := FPing("web-.*")
	if err != nil {
		t.Fatalf("FPing: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got.Results))
	}
}

func TestFPing_CommaSeparatedTargets(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01", "db-01", "web-02"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"pong": true},
			"db-01":  map[string]interface{}{"pong": true},
		},
	}
	sub2 := mockTestPing(t, nc, want)
	defer sub2.Unsubscribe()

	got, err := FPing("web-01,db-01")
	if err != nil {
		t.Fatalf("FPing: %v", err)
	}
	if got.Results == nil {
		t.Fatal("expected non-nil results")
	}
}

func TestFPing_NoMatchingTargets(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"db-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{Results: map[string]interface{}{}}
	sub2 := mockTestPing(t, nc, want)
	defer sub2.Unsubscribe()

	got, err := FPing("web-.*")
	if err != nil {
		t.Fatalf("FPing: %v", err)
	}
	if len(got.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got.Results))
	}
}

func TestFPing_NATSConnNil(t *testing.T) {
	old := pkiclient.NatsConn
	pkiclient.NatsConn = nil
	defer func() { pkiclient.NatsConn = old }()

	_, err := FPing("web-01")
	if err == nil {
		t.Fatal("expected error for nil NatsConn")
	}
}

func TestFPing_PKIListError(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

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

	_, err = FPing("web-01")
	if err == nil {
		t.Fatal("expected error from PKI list failure")
	}
}

func TestFPing_PingError(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	reply := natsReply{Error: "sprout unreachable"}
	sub2, err := nc.Subscribe("grlx.api.test.ping", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Unsubscribe()
	nc.Flush()

	_, err = FPing("web-01")
	if err == nil {
		t.Fatal("expected error from test.ping failure")
	}
}

func TestFPing_InvalidJSONResponse(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	reply := natsReply{Result: json.RawMessage(`"not an object"`)}
	sub2, err := nc.Subscribe("grlx.api.test.ping", func(msg *nats.Msg) {
		payload, _ := json.Marshal(reply)
		_ = msg.Respond(payload)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Unsubscribe()
	nc.Flush()

	_, err = FPing("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestFPing_NATSTimeout(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01"})
	defer sub1.Unsubscribe()

	// No handler for test.ping — will timeout
	oldTimeout := pkiclient.NatsRequestTimeout
	pkiclient.NatsRequestTimeout = 100 * time.Millisecond
	defer func() { pkiclient.NatsRequestTimeout = oldTimeout }()

	_, err := FPing("web-01")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFPing_RegexTarget(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{"web-01", "web-02", "db-01"})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{
		Results: map[string]interface{}{
			"web-01": map[string]interface{}{"pong": true},
			"web-02": map[string]interface{}{"pong": true},
		},
	}
	sub2 := mockTestPing(t, nc, want)
	defer sub2.Unsubscribe()

	got, err := FPing("web-.*")
	if err != nil {
		t.Fatalf("FPing: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got.Results))
	}
}

func TestFPing_EmptyAccepted(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	sub1 := mockPKIList(t, nc, []string{})
	defer sub1.Unsubscribe()

	want := apitypes.TargetedResults{Results: map[string]interface{}{}}
	sub2 := mockTestPing(t, nc, want)
	defer sub2.Unsubscribe()

	got, err := FPing("web-01")
	if err != nil {
		t.Fatalf("FPing: %v", err)
	}
	if len(got.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got.Results))
	}
}
