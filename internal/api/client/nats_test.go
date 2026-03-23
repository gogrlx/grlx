package client

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/pki"
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

func TestConnectNats_NoConfig(t *testing.T) {
	// ConnectNats will fail without proper config/auth setup
	// Just verify it returns a meaningful error
	old := NatsConn
	defer func() { NatsConn = old }()

	err := ConnectNats()
	if err == nil {
		// If it somehow succeeded (unlikely without config), clean up
		if NatsConn != nil {
			NatsConn.Close()
		}
		return
	}
	// Error is expected — connection should fail without farmer config
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

// mockBadJSONHandler subscribes to subject and replies with a valid envelope
// but invalid JSON in the result field — used to test unmarshal error paths.
func mockBadJSONHandler(t *testing.T, nc *nats.Conn, subject string) *nats.Subscription {
	t.Helper()
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		// Valid envelope, but result is not valid for the expected type
		payload := []byte(`{"result":"not-an-object"}`)
		if err := msg.Respond(payload); err != nil {
			t.Errorf("mock respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	nc.Flush()
	return sub
}

func TestGetVersion_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.version")

	_, err := GetVersion()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListSprouts_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.sprouts.list")

	_, err := ListSprouts()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetSprout_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.sprouts.get")

	_, err := GetSprout("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetSproutProps_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.props.getall")

	_, err := GetSproutProps("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestWhoAmI_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.whoami")

	_, err := WhoAmI()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestExplainAccess_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.explain")

	_, err := ExplainAccess()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListUsers_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.users")

	_, err := ListUsers()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestAddUser_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.users.add")

	_, err := AddUser("KEY", "role")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRemoveUser_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.auth.users.remove")

	_, err := RemoveUser("KEY")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetCohort_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.get")

	_, err := GetCohort("test")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestResolveCohort_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.resolve")

	_, err := ResolveCohort("test")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRefreshCohort_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.refresh")

	_, err := RefreshCohort("test")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRefreshAllCohorts_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.refresh")

	_, err := RefreshAllCohorts()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListJobs_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.jobs.list")

	_, err := ListJobs(10, "")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetJob_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.jobs.get")

	_, err := GetJob("jid-1")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListJobsForSprout_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.jobs.forsprout")

	_, err := ListJobsForSprout("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListKeys_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.pki.list")

	_, err := ListKeys()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListAuditDates_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.audit.dates")

	_, err := ListAuditDates()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestQueryAudit_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.audit.query")

	_, err := QueryAudit(audit.QueryParams{})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- Cook tests ---

func TestCook_HappyPath(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := pki.KeysByType{
		Accepted:   pki.KeySet{Sprouts: []pki.KeyManager{{SproutID: "web-01"}}},
		Unaccepted: pki.KeySet{Sprouts: []pki.KeyManager{}},
		Denied:     pki.KeySet{Sprouts: []pki.KeyManager{}},
		Rejected:   pki.KeySet{Sprouts: []pki.KeyManager{}},
	}
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	want := apitypes.CmdCook{
		JID:    "jid-cook-001",
		Recipe: "web.sls",
	}
	mockHandler(t, NatsConn, "grlx.api.cook", want)

	cmd := apitypes.CmdCook{Recipe: "web.sls"}
	got, err := Cook("web-01", cmd)
	if err != nil {
		t.Fatalf("Cook: %v", err)
	}
	if got.JID != "jid-cook-001" {
		t.Fatalf("expected JID jid-cook-001, got %q", got.JID)
	}
}

func TestCook_TargetResolveFails(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.pki.list", "connection refused")

	cmd := apitypes.CmdCook{Recipe: "web.sls"}
	_, err := Cook("web-01", cmd)
	if err == nil {
		t.Fatal("expected error from target resolution")
	}
}

func TestCook_BadJSONResponse(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := pki.KeysByType{
		Accepted:   pki.KeySet{Sprouts: []pki.KeyManager{{SproutID: "web-01"}}},
		Unaccepted: pki.KeySet{Sprouts: []pki.KeyManager{}},
		Denied:     pki.KeySet{Sprouts: []pki.KeyManager{}},
		Rejected:   pki.KeySet{Sprouts: []pki.KeyManager{}},
	}
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockBadJSONHandler(t, NatsConn, "grlx.api.cook")

	cmd := apitypes.CmdCook{Recipe: "web.sls"}
	_, err := Cook("web-01", cmd)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}
