package natsapi

import (
	"encoding/json"
	"testing"
)

func TestExtractTargetedSproutIDs(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		want    []string
		wantErr bool
	}{
		{
			name:   "single target",
			params: `{"target":[{"sprout_id":"web-1"}],"action":{}}`,
			want:   []string{"web-1"},
		},
		{
			name:   "multiple targets",
			params: `{"target":[{"sprout_id":"web-1"},{"sprout_id":"web-2"}],"action":{}}`,
			want:   []string{"web-1", "web-2"},
		},
		{
			name:    "empty targets",
			params:  `{"target":[],"action":{}}`,
			wantErr: true,
		},
		{
			name:    "missing sprout_id",
			params:  `{"target":[{"sprout_id":""}],"action":{}}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			params:  `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractTargetedSproutIDs(json.RawMessage(tt.params))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, id, tt.want[i])
				}
			}
		})
	}
}

func TestExtractShellSproutID(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		want    []string
		wantErr bool
	}{
		{
			name:   "valid sprout_id",
			params: `{"sprout_id":"db-1","cols":80,"rows":24}`,
			want:   []string{"db-1"},
		},
		{
			name:    "empty sprout_id",
			params:  `{"sprout_id":""}`,
			wantErr: true,
		},
		{
			name:    "missing sprout_id",
			params:  `{"cols":80}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractShellSproutID(json.RawMessage(tt.params))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) || got[0] != tt.want[0] {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractPropsSproutID(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		want    []string
		wantErr bool
	}{
		{
			name:   "valid",
			params: `{"sprout_id":"app-1","name":"os","value":"linux"}`,
			want:   []string{"app-1"},
		},
		{
			name:    "empty",
			params:  `{"sprout_id":"","name":"os"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPropsSproutID(json.RawMessage(tt.params))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got[0] != tt.want[0] {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractJobsForSproutID(t *testing.T) {
	got, err := extractJobsForSproutID(json.RawMessage(`{"sprout_id":"worker-3"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "worker-3" {
		t.Fatalf("got %v, want [worker-3]", got)
	}

	_, err = extractJobsForSproutID(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty sprout_id")
	}
}

func TestExtractSproutsGetID(t *testing.T) {
	got, err := extractSproutsGetID(json.RawMessage(`{"sprout_id":"api-1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "api-1" {
		t.Fatalf("got %v, want [api-1]", got)
	}
}

func TestScopeExtractorsRegistry(t *testing.T) {
	// Verify that all methods expected to have scope extractors are registered.
	expected := []string{
		"cook", "cmd.run", "test.ping", "shell.start",
		"props.getall", "props.get", "props.set", "props.delete",
		"jobs.forsprout", "sprouts.get",
	}
	for _, method := range expected {
		if _, ok := scopeExtractors[method]; !ok {
			t.Errorf("method %q is missing from scopeExtractors", method)
		}
	}

	// Verify that global/unscoped methods do NOT have extractors.
	unscoped := []string{
		"version", "pki.list", "pki.accept", "auth.whoami",
		"sprouts.list", "jobs.list", "jobs.get",
		"cohorts.list", "cohorts.get",
	}
	for _, method := range unscoped {
		if _, ok := scopeExtractors[method]; ok {
			t.Errorf("method %q should NOT have a scope extractor (it is global/unscoped)", method)
		}
	}
}

func TestAuthMiddleware_ScopeEnforcement(t *testing.T) {
	// When dangerously_allow_root is off and token is invalid,
	// scope-checked methods should still be denied at the token stage.
	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "ok", nil
	}

	wrapped := authMiddleware("cook", inner)
	params := json.RawMessage(`{"target":[{"sprout_id":"web-1"}],"action":{},"token":"invalid"}`)
	_, err := wrapped(params)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if called {
		t.Fatal("handler should not be called when token is invalid")
	}
}
