package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/gogrlx/grlx/v2/internal/props"
)

// newPropsRouter returns a mux.Router with the props routes registered,
// without auth/logging middleware so tests can call them directly.
func newPropsRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/props/{sproutID}", GetAllProps).Methods(http.MethodGet)
	r.HandleFunc("/props/{sproutID}/{name}", GetProp).Methods(http.MethodGet)
	r.HandleFunc("/props/{sproutID}/{name}", SetProp).Methods(http.MethodPut)
	r.HandleFunc("/props/{sproutID}/{name}", DeleteProp).Methods(http.MethodDelete)
	return r
}

func TestSetAndGetProp(t *testing.T) {
	router := newPropsRouter()

	// Set a property
	body, _ := json.Marshal(propRequest{Value: "hello"})
	req := httptest.NewRequest(http.MethodPut, "/props/sprout-1/greeting", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SetProp: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Get the property
	req = httptest.NewRequest(http.MethodGet, "/props/sprout-1/greeting", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetProp: expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetProp: failed to unmarshal: %v", err)
	}
	if resp["value"] != "hello" {
		t.Errorf("GetProp: expected value 'hello', got %q", resp["value"])
	}
	if resp["sproutID"] != "sprout-1" {
		t.Errorf("GetProp: expected sproutID 'sprout-1', got %q", resp["sproutID"])
	}
	if resp["name"] != "greeting" {
		t.Errorf("GetProp: expected name 'greeting', got %q", resp["name"])
	}
}

func TestGetAllProps(t *testing.T) {
	router := newPropsRouter()

	// Set multiple properties
	for _, kv := range []struct{ k, v string }{
		{"os", "linux"},
		{"arch", "amd64"},
	} {
		body, _ := json.Marshal(propRequest{Value: kv.v})
		req := httptest.NewRequest(http.MethodPut, "/props/sprout-all/"+kv.k, bytes.NewReader(body))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("SetProp %s: expected 200, got %d", kv.k, rr.Code)
		}
	}

	// Get all
	req := httptest.NewRequest(http.MethodGet, "/props/sprout-all", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetAllProps: expected 200, got %d", rr.Code)
	}
	var allProps map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &allProps); err != nil {
		t.Fatalf("GetAllProps: failed to unmarshal: %v", err)
	}
	if allProps["os"] != "linux" {
		t.Errorf("expected os=linux, got %v", allProps["os"])
	}
	if allProps["arch"] != "amd64" {
		t.Errorf("expected arch=amd64, got %v", allProps["arch"])
	}
}

func TestGetAllPropsEmpty(t *testing.T) {
	router := newPropsRouter()

	req := httptest.NewRequest(http.MethodGet, "/props/no-such-sprout-xyz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetAllProps empty: expected 200, got %d", rr.Code)
	}
	var allProps map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &allProps); err != nil {
		t.Fatalf("GetAllProps empty: failed to unmarshal: %v", err)
	}
	if len(allProps) != 0 {
		t.Errorf("expected empty map, got %v", allProps)
	}
}

func TestDeletePropHandler(t *testing.T) {
	router := newPropsRouter()

	// Set then delete
	body, _ := json.Marshal(propRequest{Value: "temporary"})
	req := httptest.NewRequest(http.MethodPut, "/props/sprout-del/temp", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("SetProp: expected 200, got %d", rr.Code)
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/props/sprout-del/temp", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("DeleteProp: expected 200, got %d", rr.Code)
	}

	// Verify gone
	req = httptest.NewRequest(http.MethodGet, "/props/sprout-del/temp", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["value"] != "" {
		t.Errorf("expected empty value after delete, got %q", resp["value"])
	}
}

func TestDeletePropNonExistent(t *testing.T) {
	router := newPropsRouter()

	req := httptest.NewRequest(http.MethodDelete, "/props/sprout-nope/nokey", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("DeleteProp non-existent: expected 200, got %d", rr.Code)
	}
}

func TestSetPropInvalidBody(t *testing.T) {
	router := newPropsRouter()

	req := httptest.NewRequest(http.MethodPut, "/props/sprout-1/key", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("SetProp bad body: expected 400, got %d", rr.Code)
	}
}

func TestGetPropMissing(t *testing.T) {
	router := newPropsRouter()

	req := httptest.NewRequest(http.MethodGet, "/props/no-sprout/no-key", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetProp missing: expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["value"] != "" {
		t.Errorf("expected empty value for missing prop, got %q", resp["value"])
	}
}

func TestSetPropOverwrite(t *testing.T) {
	router := newPropsRouter()

	body, _ := json.Marshal(propRequest{Value: "old"})
	req := httptest.NewRequest(http.MethodPut, "/props/sprout-ow/key", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first set: expected 200, got %d", rr.Code)
	}

	body, _ = json.Marshal(propRequest{Value: "new"})
	req = httptest.NewRequest(http.MethodPut, "/props/sprout-ow/key", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("second set: expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/props/sprout-ow/key", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["value"] != "new" {
		t.Errorf("expected 'new' after overwrite, got %q", resp["value"])
	}
}

// TestExportedPropsFunctions verifies the exported wrapper functions work.
func TestExportedPropsFunctions(t *testing.T) {
	// Direct calls to the exported props package functions
	err := props.SetProp("test-export", "key1", "val1")
	if err != nil {
		t.Fatalf("SetProp: %v", err)
	}
	got := props.GetStringProp("test-export", "key1")
	if got != "val1" {
		t.Errorf("GetStringProp: expected 'val1', got %q", got)
	}
	all := props.GetProps("test-export")
	if all["key1"] != "val1" {
		t.Errorf("GetProps: expected key1=val1, got %v", all)
	}
	err = props.DeleteProp("test-export", "key1")
	if err != nil {
		t.Fatalf("DeleteProp: %v", err)
	}
	got = props.GetStringProp("test-export", "key1")
	if got != "" {
		t.Errorf("expected empty after delete, got %q", got)
	}
}
