package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
)

func TestCook_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestCook_InvalidTargetAction(t *testing.T) {
	// Valid TargetedAction JSON but the action field is a string, not an object
	// This should fail when trying to decode as CmdCook
	body := []byte(`{"target":[{"id":"sprout1"}],"action":"not-an-object"}`)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d", w.Code)
	}
}

func TestCook_InvalidSproutID(t *testing.T) {
	dir := setupPKIDirs(t)
	_ = dir

	// Build JSON manually to control the action structure
	body := []byte(`{"target":[{"id":"INVALID-ID"}],"action":{"recipe":"test-recipe"}}`)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sprout ID, got %d", w.Code)
	}
}

func TestCook_SproutIDWithUnderscore(t *testing.T) {
	setupPKIDirs(t)

	body := []byte(`{"target":[{"id":"bad_underscore"}],"action":{"recipe":"test-recipe"}}`)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for underscore in sprout ID, got %d", w.Code)
	}
}

func TestCook_UnknownSprout(t *testing.T) {
	setupPKIDirs(t) // empty PKI

	body := []byte(`{"target":[{"id":"unknown-sprout"}],"action":{"recipe":"test-recipe"}}`)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown sprout, got %d", w.Code)
	}

	var results apitypes.TargetedResults
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if results.Results != nil {
		t.Error("expected nil results for unknown sprout")
	}
}

func TestCook_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader([]byte("")))
	w := httptest.NewRecorder()

	Cook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestCook_NilActionFields(t *testing.T) {
	// Valid JSON but action has no recipe field
	ta := apitypes.TargetedAction{
		Target: nil,
		Action: map[string]interface{}{},
	}
	body, _ := json.Marshal(ta)
	req := httptest.NewRequest(http.MethodPost, "/cook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Cook(w, req)

	// Should succeed — no targets means the loop is skipped and JID is returned
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterNatsConn(t *testing.T) {
	// Verify RegisterNatsConn sets the package-level conn variable
	oldConn := conn
	defer func() { conn = oldConn }()

	RegisterNatsConn(nil)
	if conn != nil {
		t.Error("expected nil conn after registering nil")
	}
}
