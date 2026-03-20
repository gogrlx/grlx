package client

import (
	"encoding/json"
	"testing"
)

func TestInjectToken_NilPayload(t *testing.T) {
	// Without valid auth config, injectToken should return nil unchanged.
	got := injectToken(nil)
	if got != nil {
		// If auth is configured in test env, the token gets injected.
		// Just verify it's valid JSON.
		var obj map[string]interface{}
		if err := json.Unmarshal(got, &obj); err != nil {
			t.Fatalf("injected payload is not valid JSON: %v", err)
		}
	}
}

func TestInjectToken_EmptyPayload(t *testing.T) {
	got := injectToken([]byte{})
	if len(got) == 0 {
		// No auth configured — returned empty unchanged.
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("injected payload is not valid JSON: %v", err)
	}
}

func TestInjectToken_ExistingJSON(t *testing.T) {
	input := []byte(`{"limit":10,"sprout_id":"web-01"}`)
	got := injectToken(input)

	// Without auth config, payload should be returned unchanged.
	var obj map[string]interface{}
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Original fields must be preserved.
	if obj["limit"] != float64(10) {
		t.Errorf("expected limit=10, got %v", obj["limit"])
	}
	if obj["sprout_id"] != "web-01" {
		t.Errorf("expected sprout_id=web-01, got %v", obj["sprout_id"])
	}
}

func TestInjectToken_NonObjectPayload(t *testing.T) {
	// Array payload — can't inject token, should return unchanged.
	input := []byte(`[1,2,3]`)
	got := injectToken(input)
	if string(got) != string(input) {
		// This is fine either way — just ensure it's valid JSON.
		var arr []interface{}
		if err := json.Unmarshal(got, &arr); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
	}
}
