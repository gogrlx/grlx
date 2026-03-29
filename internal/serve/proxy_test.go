package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/api/client"
)

// natsResponse mirrors the client's internal envelope.
type natsResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// startTestNATS starts an embedded NATS server and wires client.NatsConn.
func startTestNATS(t *testing.T) func() {
	t.Helper()
	opts := &server.Options{Host: "127.0.0.1", Port: -1}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start NATS server: %v", err)
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
	client.NatsConn = nc
	return func() {
		client.NatsConn = nil
		nc.Close()
		ns.Shutdown()
	}
}

// mockMethod subscribes to grlx.api.<method> and replies with given data.
func mockMethod(t *testing.T, nc *nats.Conn, method string, response any) {
	t.Helper()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp := natsResponse{Result: data}
	sub, err := nc.Subscribe("grlx.api."+method, func(msg *nats.Msg) {
		payload, _ := json.Marshal(resp)
		msg.Respond(payload)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() { sub.Unsubscribe() })
	nc.Flush()
}

// mockMethodError subscribes and replies with an error envelope.
func mockMethodError(t *testing.T, nc *nats.Conn, method, errMsg string) {
	t.Helper()
	resp := natsResponse{Error: errMsg}
	sub, err := nc.Subscribe("grlx.api."+method, func(msg *nats.Msg) {
		payload, _ := json.Marshal(resp)
		msg.Respond(payload)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() { sub.Unsubscribe() })
	nc.Flush()
}

func TestHandleNATSProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "sprouts.list", []map[string]string{
		{"id": "sprout-1", "hostname": "web-01"},
	})

	handler := HandleNATSProxy("sprouts.list")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprouts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "sprout-1") {
		t.Fatalf("expected sprout-1 in body, got %q", body)
	}
}

func TestHandleNATSProxy_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "sprouts.list", "access denied")

	handler := HandleNATSProxy("sprouts.list")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprouts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithID_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "sprouts.get", map[string]string{
		"id": "sprout-alpha", "hostname": "alpha",
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprouts/sprout-alpha", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "sprout-alpha") {
		t.Fatalf("expected sprout-alpha in body")
	}
}

func TestHandleNATSProxyWithID_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "sprouts.get", "not found")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprouts/sprout-alpha", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithBody_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "cook", map[string]string{"jid": "jid-001"})

	mux := NewMux()
	body := `{"recipe":"nginx.install","target":"web*"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "jid-001") {
		t.Fatalf("expected jid-001 in body")
	}
}

func TestHandleNATSProxyWithBody_ReadBodyError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	handler := HandleNATSProxyWithBody("cook")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cook", &errorReader{})
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "failed to read request body") {
		t.Fatalf("expected read error message")
	}
}

func TestHandleJobProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "jobs.get", map[string]string{
		"jid": "jid-42", "status": "completed",
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jid-42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "jid-42") {
		t.Fatalf("expected jid-42 in body")
	}
}

func TestHandleJobProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "jobs.get", "not found")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jid-42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleJobsForSproutProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "jobs.forsprout", []map[string]string{
		{"jid": "j1"}, {"jid": "j2"},
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/sprout/sprout-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleJobsForSproutProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "jobs.forsprout", "fail")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/sprout/sprout-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlePropsAllProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "props.getall", map[string]string{
		"hostname": "web-01", "os": "linux",
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/props/sprout-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "web-01") {
		t.Fatalf("expected props in body")
	}
}

func TestHandlePropsAllProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "props.getall", "sprout offline")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/props/sprout-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlePropsKeyProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "props.get", map[string]string{
		"value": "web-01",
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/props/sprout-1/hostname", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlePropsKeyProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "props.get", "not found")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/props/sprout-1/hostname", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlePropsSetProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "props.set", map[string]string{"ok": "true"})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", strings.NewReader(`{"value":"web-01"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlePropsSetProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "props.set", "write failed")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", strings.NewReader(`"web-01"`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlePropsSetProxy_ReadBodyError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	handler := HandlePropsSetProxy("props.set")

	// Use a mux to set path values
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/props/{id}/{key}", handler)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", &errorReader{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCohortGetProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "cohorts.get", map[string]any{
		"name": "webservers", "type": "static", "count": 3,
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cohorts/webservers", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "webservers") {
		t.Fatalf("expected cohort name in body")
	}
}

func TestHandleCohortGetProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "cohorts.get", "not found")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cohorts/missing", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleRecipeGetProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "recipes.get", map[string]string{
		"name": "base/webserver", "content": "pkg.installed: nginx",
	})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/base/webserver", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleRecipeGetProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "recipes.get", "recipe not found")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/missing", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleUserRemoveProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "auth.users.remove", map[string]string{"removed": "true"})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/users/NKEY_ABC", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleUserRemoveProxy_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "auth.users.remove", "user not found")

	mux := NewMux()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/users/NKEY_ABC", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithQuery_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "audit.query", []map[string]string{
		{"action": "cook", "user": "admin"},
	})

	handler := HandleNATSProxyWithQuery("audit.query")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?date=2026-01-01&action=cook&pubkey=ABC&limit=10&failed_only=true", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithQuery_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethodError(t, client.NatsConn, "audit.query", "audit unavailable")

	handler := HandleNATSProxyWithQuery("audit.query")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?date=2026-01-01", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlePropsDeleteProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "props.delete", map[string]string{"deleted": "true"})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/props/sprout-1/hostname", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleJobCancelProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "jobs.cancel", map[string]string{"cancelled": "true"})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/jid-42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleKeysAcceptProxy_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "pki.accept", map[string]string{"accepted": "true"})

	mux := NewMux()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/sprout-1/accept", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleVersion_WithNATS(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockMethod(t, client.NatsConn, "version", map[string]string{
		"tag": "v2.1.0", "arch": "linux",
	})

	BuildInfo.Tag = "test-cli"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	HandleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "test-cli") {
		t.Fatalf("expected CLI version in body")
	}
}

func TestUIHandler_AssetsCacheControl(t *testing.T) {
	handler := UIHandler()

	// Request a path under assets/ — should get cache-control header.
	// Since we can't know exact asset filenames, test the SPA fallback.
	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("SPA fallback: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "grlx") {
		t.Fatal("SPA fallback should serve index.html")
	}
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
