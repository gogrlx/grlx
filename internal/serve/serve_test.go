package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
}

func TestUIHandlerServesEmbeddedUI(t *testing.T) {
	handler := UIHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
	if !strings.Contains(body, "grlx") {
		t.Fatal("expected embedded UI to contain 'grlx'")
	}
}

func TestHandleVersion(t *testing.T) {
	BuildInfo.Tag = "test"
	BuildInfo.Arch = "linux"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()

	HandleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := body["cli"]; !ok {
		t.Fatal("expected 'cli' key in version response")
	}
}

func TestWithCORS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := WithCORS(inner)

	// Test preflight
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rec.Code)
	}
	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Fatalf("expected CORS origin *, got %q", origin)
	}

	// Test normal request has CORS headers
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Fatalf("expected CORS origin *, got %q", origin)
	}
}

func TestHandleNATSProxyNoConnection(t *testing.T) {
	handler := HandleNATSProxy("test.method")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusCreated, map[string]int{"count": 42})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var body map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["count"] != 42 {
		t.Fatalf("expected count 42, got %d", body["count"])
	}
}

func TestNewMux(t *testing.T) {
	mux := NewMux()
	if mux == nil {
		t.Fatal("NewMux returned nil")
	}

	// Health endpoint should work through the mux
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from mux health, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithBodyInvalidJSON(t *testing.T) {
	handler := HandleNATSProxyWithBody("test.method")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", strings.NewReader("not valid json"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "invalid JSON body" {
		t.Fatalf("expected 'invalid JSON body' error, got %q", body["error"])
	}
}

func TestHandleNATSProxyWithBodyNoConnection(t *testing.T) {
	handler := HandleNATSProxyWithBody("test.method")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", strings.NewReader(`{"key":"value"}`))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithBodyEmptyBody(t *testing.T) {
	handler := HandleNATSProxyWithBody("test.method")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", strings.NewReader(""))
	rec := httptest.NewRecorder()

	handler(rec, req)

	// With no NATS connection, should get 502 (params is nil, request goes through)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestHandlePropsKeyProxyMissingParams(t *testing.T) {
	handler := HandlePropsKeyProxy("props.get")

	// Missing both id and key (handler expects path values from mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/props//", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandlePropsKeyProxyNoConnection(t *testing.T) {
	mux := NewMux()

	// DELETE only exists for the key variant
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/props/sprout-1/hostname", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestHandlePropsAllProxyMissingID(t *testing.T) {
	handler := HandlePropsAllProxy("props.getall")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/props/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleJobProxyMissingJID(t *testing.T) {
	handler := HandleJobProxy("jobs.get")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleJobsForSproutProxyMissingID(t *testing.T) {
	handler := HandleJobsForSproutProxy("jobs.forsprout")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/sprout/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandlePropsSetProxyNoConnection(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", strings.NewReader(`"my-host"`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// Should reach NATS and get 502 (no connection)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestMuxJobsSproutRoute(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/sprout/sprout-alpha", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// No NATS connection, should get 502
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestMuxJobsCancelRoute(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/jid-001", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestMuxKeysRoutes(t *testing.T) {
	mux := NewMux()

	// List keys
	req := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("keys list: expected 502, got %d", rec.Code)
	}

	// Accept key
	req = httptest.NewRequest(http.MethodPost, "/api/v1/keys/sprout-1/accept", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("keys accept: expected 502, got %d", rec.Code)
	}

	// Delete key
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/keys/sprout-1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("keys delete: expected 502, got %d", rec.Code)
	}
}

func TestMuxAuthRoutes(t *testing.T) {
	mux := NewMux()

	// Whoami
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("whoami: expected 502, got %d", rec.Code)
	}

	// Users list
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/users", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("users: expected 502, got %d", rec.Code)
	}
}

func TestMuxCookRoute(t *testing.T) {
	mux := NewMux()

	body := `{"target":"web*","action":{"recipe":"nginx.install","test":false}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("cook: expected 502, got %d", rec.Code)
	}
}

func TestHandleOpenAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	HandleOpenAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-yaml" {
		t.Fatalf("expected Content-Type application/x-yaml, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "openapi: 3.1.0") {
		t.Fatal("expected OpenAPI 3.1 spec in response body")
	}
	if !strings.Contains(body, "grlx CLI API") {
		t.Fatal("expected API title in response body")
	}
}

func TestHandleOpenAPIViaMux(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "openapi:") {
		t.Fatal("expected openapi spec from mux route")
	}
}

func TestHandleCohortGetProxyMissingName(t *testing.T) {
	handler := HandleCohortGetProxy("cohorts.get")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cohorts/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "missing cohort name" {
		t.Fatalf("expected 'missing cohort name' error, got %q", body["error"])
	}
}

func TestMuxCohortGetRoute(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cohorts/webservers", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// No NATS connection, should get 502
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("cohort get: expected 502, got %d", rec.Code)
	}
}

func TestMuxAuthExplainRoute(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/explain", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// No NATS connection, should get 502
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("auth explain: expected 502, got %d", rec.Code)
	}
}

func TestMuxCohortsResolveRoute(t *testing.T) {
	mux := NewMux()

	body := `{"name":"webservers"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cohorts/resolve", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("cohorts resolve: expected 502, got %d", rec.Code)
	}
}

func TestMuxCohortsRefreshRoute(t *testing.T) {
	mux := NewMux()

	// Refresh all cohorts (empty body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cohorts/refresh", strings.NewReader(""))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("cohorts refresh (all): expected 502, got %d", rec.Code)
	}

	// Refresh a single cohort
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cohorts/refresh", strings.NewReader(`{"name":"web"}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("cohorts refresh (single): expected 502, got %d", rec.Code)
	}
}

func TestMuxCohortsListRoute(t *testing.T) {
	mux := NewMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cohorts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("cohorts list: expected 502, got %d", rec.Code)
	}
}

func TestMuxRecipesRoutes(t *testing.T) {
	mux := NewMux()

	// List recipes
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("recipes list: expected 502, got %d", rec.Code)
	}

	// Get a recipe by name
	req = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/base/webserver", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("recipes get: expected 502, got %d", rec.Code)
	}
}

func TestMuxAuditRoutes(t *testing.T) {
	mux := NewMux()

	// List audit dates
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/dates", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("audit dates: expected 502, got %d", rec.Code)
	}

	// Query audit log
	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit?date=2025-01-01&action=cook", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("audit query: expected 502, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithQueryParams(t *testing.T) {
	handler := HandleNATSProxyWithQuery("audit.query")

	// With query parameters
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?date=2025-03-15&action=cook&pubkey=ABC&limit=10&failed_only=true", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	// No NATS connection, should get 502
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}

func TestHandleNATSProxyWithIDMissingParam(t *testing.T) {
	handler := HandleNATSProxyWithID("sprouts.get")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprouts/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleRecipeGetProxyMissingName(t *testing.T) {
	handler := HandleRecipeGetProxy("recipes.get")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandlePropsSetProxyFormats(t *testing.T) {
	mux := NewMux()

	// JSON wrapper format: {"value": "my-host"}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", strings.NewReader(`{"value":"my-host"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("props set wrapper: expected 502, got %d", rec.Code)
	}

	// Bare JSON string format: "my-host"
	req = httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", strings.NewReader(`"my-host"`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("props set bare: expected 502, got %d", rec.Code)
	}

	// Raw text format: my-host
	req = httptest.NewRequest(http.MethodPut, "/api/v1/props/sprout-1/hostname", strings.NewReader(`my-host`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("props set raw: expected 502, got %d", rec.Code)
	}
}
