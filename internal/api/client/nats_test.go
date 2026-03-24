package client

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// startTestNATS starts an embedded NATS server and connects NatsConn to it.
// Returns a cleanup function that must be deferred.
func startTestNATS(t *testing.T) func() {
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

	NatsConn = nc
	return func() {
		NatsConn = nil
		nc.Close()
		ns.Shutdown()
	}
}

// mockHandler subscribes to subject and replies with the given JSON body.
// Useful for happy-path tests.
func mockHandler(t *testing.T, nc *nats.Conn, subject string, response interface{}) *nats.Subscription {
	t.Helper()
	resp := natsResponse{}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal mock response: %v", err)
	}
	resp.Result = data

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		payload, _ := json.Marshal(resp)
		if err := msg.Respond(payload); err != nil {
			t.Errorf("mock respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe mock handler: %v", err)
	}
	nc.Flush()
	return sub
}

// mockErrorHandler subscribes to subject and replies with an error envelope.
func mockErrorHandler(t *testing.T, nc *nats.Conn, subject, errMsg string) *nats.Subscription {
	t.Helper()
	resp := natsResponse{Error: errMsg}

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		payload, _ := json.Marshal(resp)
		if err := msg.Respond(payload); err != nil {
			t.Errorf("mock respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe mock error handler: %v", err)
	}
	nc.Flush()
	return sub
}

// mockBadJSONHandler subscribes to subject and replies with a result that is
// not valid JSON for the expected type (forces an unmarshal error in callers).
func mockBadJSONHandler(t *testing.T, nc *nats.Conn, subject string) *nats.Subscription {
	t.Helper()
	// Return a result that is valid natsResponse JSON but whose Result
	// field contains a string instead of the expected object/array.
	resp := natsResponse{Result: json.RawMessage(`"not an object"`)}

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		payload, _ := json.Marshal(resp)
		if err := msg.Respond(payload); err != nil {
			t.Errorf("mock respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe mock bad json handler: %v", err)
	}
	nc.Flush()
	return sub
}

// --- NatsRequest tests ---

func TestNatsRequest_NilConn(t *testing.T) {
	old := NatsConn
	NatsConn = nil
	defer func() { NatsConn = old }()

	_, err := NatsRequest("version", nil)
	if err == nil {
		t.Fatal("expected error for nil NatsConn")
	}
	if err.Error() != "NATS connection not established" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNatsRequest_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := map[string]string{"tag": "v2.0.0"}
	mockHandler(t, NatsConn, "grlx.api.version", want)

	result, err := NatsRequest("version", nil)
	if err != nil {
		t.Fatalf("NatsRequest: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got["tag"] != "v2.0.0" {
		t.Fatalf("expected tag v2.0.0, got %q", got["tag"])
	}
}

func TestNatsRequest_ErrorResponse(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.fail", "something went wrong")

	_, err := NatsRequest("fail", nil)
	if err == nil {
		t.Fatal("expected error from error response")
	}
	if err.Error() != "something went wrong" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNatsRequest_WithParams(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	// Subscribe and verify params are received correctly
	sub, err := NatsConn.Subscribe("grlx.api.echo", func(msg *nats.Msg) {
		resp := natsResponse{Result: msg.Data}
		payload, _ := json.Marshal(resp)
		if err := msg.Respond(payload); err != nil {
			t.Errorf("mock respond: %v", err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	NatsConn.Flush()

	params := map[string]string{"name": "web-servers"}
	result, err := NatsRequest("echo", params)
	if err != nil {
		t.Fatalf("NatsRequest: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["name"] != "web-servers" {
		t.Fatalf("expected name web-servers, got %q", got["name"])
	}
}

func TestNatsRequest_Timeout(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	// Set a short timeout and don't register any handler
	old := NatsRequestTimeout
	NatsRequestTimeout = 50 * time.Millisecond
	defer func() { NatsRequestTimeout = old }()

	_, err := NatsRequest("nonexistent.method", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestNatsRequest_InvalidResponse(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	// Reply with invalid JSON
	sub, err := NatsConn.Subscribe("grlx.api.badjson", func(msg *nats.Msg) {
		if err := msg.Respond([]byte("not json")); err != nil {
			t.Errorf("mock respond: %v", err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	NatsConn.Flush()

	_, err = NatsRequest("badjson", nil)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}
