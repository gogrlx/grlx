package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/config"
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
