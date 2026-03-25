package cmd

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

func setupCmdTestPKI(t *testing.T) string {
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

func addCmdTestSprout(t *testing.T, dir, state, id, nkey string) {
	t.Helper()
	path := filepath.Join(dir, "sprouts", state, id)
	if err := os.WriteFile(path, []byte(nkey), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHCmdRun_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHCmdRun_InvalidAction(t *testing.T) {
	// Valid TargetedAction but action is a string, not a CmdRun object
	body := []byte(`{"target":[{"id":"sprout1"}],"action":"not-a-cmd-object"}`)
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHCmdRun_InvalidSproutID(t *testing.T) {
	setupCmdTestPKI(t)

	body := []byte(`{"target":[{"id":"INVALID-CAPS"}],"action":{"command":"echo","args":["hello"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHCmdRun_SproutIDWithUnderscore(t *testing.T) {
	setupCmdTestPKI(t)

	body := []byte(`{"target":[{"id":"bad_underscore"}],"action":{"command":"echo","args":["hello"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHCmdRun_UnknownSprout(t *testing.T) {
	setupCmdTestPKI(t)

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{{SproutID: "unknown-sprout"}},
		Action: map[string]interface{}{"command": "echo", "args": []string{"hello"}},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHCmdRun_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader([]byte("")))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestHCmdRun_MultipleTargetsOneInvalid(t *testing.T) {
	dir := setupCmdTestPKI(t)
	addCmdTestSprout(t, dir, "accepted", "good-sprout", "UKEY1")

	ta := apitypes.TargetedAction{
		Target: []pki.KeyManager{
			{SproutID: "good-sprout"},
			{SproutID: "BAD-ID"},
		},
		Action: map[string]interface{}{"command": "echo"},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/cmd/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	HCmdRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
