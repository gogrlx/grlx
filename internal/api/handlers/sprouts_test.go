package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

func setupTestPKI(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	config.FarmerPKI = tmpDir + "/"

	for _, state := range []string{"accepted", "unaccepted", "denied", "rejected"} {
		dir := filepath.Join(tmpDir, "sprouts", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create test PKI directory: %v", err)
		}
	}
	return tmpDir
}

func addTestSprout(t *testing.T, tmpDir, state, id, nkey string) {
	t.Helper()
	path := filepath.Join(tmpDir, "sprouts", state, id)
	if err := os.WriteFile(path, []byte(nkey), 0o644); err != nil {
		t.Fatalf("failed to write test sprout key: %v", err)
	}
}

func TestListSprouts_Empty(t *testing.T) {
	setupTestPKI(t)

	req := httptest.NewRequest(http.MethodGet, "/sprouts", nil)
	w := httptest.NewRecorder()

	ListSprouts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp SproutListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Sprouts) != 0 {
		t.Errorf("expected 0 sprouts, got %d", len(resp.Sprouts))
	}
}

func TestListSprouts_WithSprouts(t *testing.T) {
	tmpDir := setupTestPKI(t)

	addTestSprout(t, tmpDir, "accepted", "web-server-1", "UABC123")
	addTestSprout(t, tmpDir, "accepted", "db-server-1", "UDEF456")
	addTestSprout(t, tmpDir, "unaccepted", "new-node", "UGHI789")
	addTestSprout(t, tmpDir, "denied", "bad-actor", "UJKL012")

	req := httptest.NewRequest(http.MethodGet, "/sprouts", nil)
	w := httptest.NewRecorder()

	ListSprouts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp SproutListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Sprouts) != 4 {
		t.Fatalf("expected 4 sprouts, got %d", len(resp.Sprouts))
	}

	// Build a map for easier assertion
	sproutMap := make(map[string]SproutInfo)
	for _, s := range resp.Sprouts {
		sproutMap[s.ID] = s
	}

	if s, ok := sproutMap["web-server-1"]; !ok {
		t.Error("expected web-server-1 in results")
	} else {
		if s.KeyState != "accepted" {
			t.Errorf("expected key_state 'accepted', got %q", s.KeyState)
		}
		if s.NKey != "UABC123" {
			t.Errorf("expected nkey 'UABC123', got %q", s.NKey)
		}
		// No NATS conn, so connected should be false
		if s.Connected {
			t.Error("expected connected=false without NATS connection")
		}
	}

	if s, ok := sproutMap["new-node"]; !ok {
		t.Error("expected new-node in results")
	} else if s.KeyState != "unaccepted" {
		t.Errorf("expected key_state 'unaccepted', got %q", s.KeyState)
	}

	if s, ok := sproutMap["bad-actor"]; !ok {
		t.Error("expected bad-actor in results")
	} else if s.KeyState != "denied" {
		t.Errorf("expected key_state 'denied', got %q", s.KeyState)
	}
}

func TestGetSprout_Found(t *testing.T) {
	tmpDir := setupTestPKI(t)
	addTestSprout(t, tmpDir, "accepted", "app-server", "UMNOP345")

	body, _ := json.Marshal(pki.KeyManager{SproutID: "app-server"})
	req := httptest.NewRequest(http.MethodPost, "/sprouts/get", bytes.NewReader(body))
	w := httptest.NewRecorder()

	GetSprout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var info SproutInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if info.ID != "app-server" {
		t.Errorf("expected id 'app-server', got %q", info.ID)
	}
	if info.KeyState != "accepted" {
		t.Errorf("expected key_state 'accepted', got %q", info.KeyState)
	}
	if info.NKey != "UMNOP345" {
		t.Errorf("expected nkey 'UMNOP345', got %q", info.NKey)
	}
}

func TestGetSprout_NotFound(t *testing.T) {
	setupTestPKI(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/sprouts/get", bytes.NewReader(body))
	w := httptest.NewRecorder()

	GetSprout(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestGetSprout_InvalidID(t *testing.T) {
	setupTestPKI(t)

	body, _ := json.Marshal(pki.KeyManager{SproutID: "INVALID-CAPS"})
	req := httptest.NewRequest(http.MethodPost, "/sprouts/get", bytes.NewReader(body))
	w := httptest.NewRecorder()

	GetSprout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetSprout_BadBody(t *testing.T) {
	setupTestPKI(t)

	req := httptest.NewRequest(http.MethodPost, "/sprouts/get", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	GetSprout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestResolveKeyState(t *testing.T) {
	tmpDir := setupTestPKI(t)

	addTestSprout(t, tmpDir, "accepted", "acc-sprout", "KEY1")
	addTestSprout(t, tmpDir, "denied", "den-sprout", "KEY2")
	addTestSprout(t, tmpDir, "rejected", "rej-sprout", "KEY3")
	addTestSprout(t, tmpDir, "unaccepted", "una-sprout", "KEY4")

	tests := []struct {
		id       string
		expected string
	}{
		{"acc-sprout", "accepted"},
		{"den-sprout", "denied"},
		{"rej-sprout", "rejected"},
		{"una-sprout", "unaccepted"},
		{"missing", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			state := resolveKeyState(tt.id)
			if state != tt.expected {
				t.Errorf("resolveKeyState(%q) = %q, want %q", tt.id, state, tt.expected)
			}
		})
	}
}

func TestListSprouts_EmptyReturnsEmptyArray(t *testing.T) {
	setupTestPKI(t)

	req := httptest.NewRequest(http.MethodGet, "/sprouts", nil)
	w := httptest.NewRecorder()

	ListSprouts(w, req)

	// Verify it's an empty array, not null
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if string(raw["sprouts"]) == "null" {
		t.Error("expected empty array [], got null")
	}
}
