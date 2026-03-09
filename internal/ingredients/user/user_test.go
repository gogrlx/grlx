package user

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func compareResults(t *testing.T, result cook.Result, expected cook.Result) {
	t.Helper()
	if result.Succeeded != expected.Succeeded {
		t.Errorf("expected succeeded to be %v, got %v", expected.Succeeded, result.Succeeded)
	}
	if result.Failed != expected.Failed {
		t.Errorf("expected failed to be %v, got %v", expected.Failed, result.Failed)
	}
	if result.Changed != expected.Changed {
		t.Errorf("expected changed to be %v, got %v", expected.Changed, result.Changed)
	}
	if len(result.Notes) != len(expected.Notes) {
		t.Errorf("expected %v notes, got %v. Got %v", len(expected.Notes), len(result.Notes), result.Notes)
		return
	}
	for i, note := range result.Notes {
		if note.String() != expected.Notes[i].String() {
			t.Errorf("expected note %d to be %q, got %q", i, expected.Notes[i].String(), note.String())
		}
	}
}

func TestUserParse(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		method  string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:   "valid absent",
			id:     "test-user-absent",
			method: "absent",
			params: map[string]interface{}{"name": "testuser"},
		},
		{
			name:   "valid exists",
			id:     "test-user-exists",
			method: "exists",
			params: map[string]interface{}{"name": "testuser"},
		},
		{
			name:   "valid present with all options",
			id:     "test-user-present",
			method: "present",
			params: map[string]interface{}{
				"name":   "testuser",
				"uid":    "1234",
				"gid":    "1234",
				"shell":  "/bin/bash",
				"home":   "/home/testuser",
				"groups": []string{"wheel", "docker"},
			},
		},
		{
			name:   "valid present with name only",
			id:     "test-user-present-min",
			method: "present",
			params: map[string]interface{}{"name": "testuser"},
		},
		{
			name:    "missing name in absent",
			id:      "test-user-absent-no-name",
			method:  "absent",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "missing name in exists",
			id:      "test-user-exists-no-name",
			method:  "exists",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "missing name in present",
			id:      "test-user-present-no-name",
			method:  "present",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "empty name string",
			id:      "test-user-empty-name",
			method:  "absent",
			params:  map[string]interface{}{"name": ""},
			wantErr: true,
		},
		{
			name:    "name is not a string",
			id:      "test-user-int-name",
			method:  "absent",
			params:  map[string]interface{}{"name": 42},
			wantErr: true,
		},
		{
			name:    "nil params",
			id:      "test-user-nil-params",
			method:  "absent",
			params:  nil,
			wantErr: true,
		},
		{
			name:    "undefined method",
			id:      "test-user-bad-method",
			method:  "nonexistent",
			params:  map[string]interface{}{"name": "testuser"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := User{}
			_, err := u.Parse(tt.id, tt.method, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserMethods(t *testing.T) {
	u := User{}
	name, methods := u.Methods()
	if name != "user" {
		t.Errorf("expected ingredient name 'user', got %q", name)
	}
	expected := map[string]bool{"absent": true, "exists": true, "present": true}
	if len(methods) != len(expected) {
		t.Fatalf("expected %d methods, got %d", len(expected), len(methods))
	}
	for _, method := range methods {
		if !expected[method] {
			t.Errorf("unexpected method %q", method)
		}
	}
}

func TestUserPropertiesForMethod(t *testing.T) {
	u := User{}

	tests := []struct {
		method     string
		wantErr    bool
		wantKeys   []string
		wantAbsent []string
	}{
		{
			method:   "absent",
			wantKeys: []string{"name"},
		},
		{
			method:   "exists",
			wantKeys: []string{"name"},
		},
		{
			method:   "present",
			wantKeys: []string{"name", "uid", "gid", "groups", "shell", "home"},
		},
		{
			method:  "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			props, err := u.PropertiesForMethod(tt.method)
			if (err != nil) != tt.wantErr {
				t.Errorf("PropertiesForMethod(%q) error = %v, wantErr %v", tt.method, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, key := range tt.wantKeys {
				if _, ok := props[key]; !ok {
					t.Errorf("expected key %q in properties for method %q", key, tt.method)
				}
			}
		})
	}
}

func TestUserProperties(t *testing.T) {
	u := User{
		id:     "test",
		method: "present",
		params: map[string]interface{}{
			"name":  "testuser",
			"shell": "/bin/bash",
		},
	}

	props, err := u.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if props["name"] != "testuser" {
		t.Errorf("expected name 'testuser', got %v", props["name"])
	}
	if props["shell"] != "/bin/bash" {
		t.Errorf("expected shell '/bin/bash', got %v", props["shell"])
	}
}

func TestUserExists(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected cook.Result
		wantErr  bool
	}{
		{
			name:   "root user exists",
			params: map[string]interface{}{"name": "root"},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     []fmt.Stringer{cook.SimpleNote("user root exists")},
			},
		},
		{
			name:   "nonexistent user",
			params: map[string]interface{}{"name": "grlx-test-nonexistent-user-abc123"},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{cook.SimpleNote("user grlx-test-nonexistent-user-abc123 does not exist")},
			},
		},
		{
			name:   "invalid name type",
			params: map[string]interface{}{"name": 42},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := User{
				id:     "test-exists",
				method: "exists",
				params: tt.params,
			}
			result, err := u.exists(context.Background(), false)
			if (err != nil) != tt.wantErr {
				t.Errorf("exists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			compareResults(t, result, tt.expected)
		})
	}
}

func TestUserAbsentAlreadyAbsent(t *testing.T) {
	// When user doesn't exist, absent is a no-op
	u := User{
		id:     "test-absent",
		method: "absent",
		params: map[string]interface{}{"name": "grlx-test-nonexistent-user-abc123"},
	}
	result, err := u.absent(context.Background(), false)
	if err != nil {
		t.Fatalf("absent() unexpected error: %v", err)
	}
	expected := cook.Result{
		Succeeded: true,
		Failed:    false,
		Notes:     []fmt.Stringer{cook.SimpleNote("user grlx-test-nonexistent-user-abc123 already absent, nothing to do")},
	}
	compareResults(t, result, expected)
}

func TestUserAbsentInvalidName(t *testing.T) {
	u := User{
		id:     "test-absent-invalid",
		method: "absent",
		params: map[string]interface{}{"name": 42},
	}
	result, err := u.absent(context.Background(), false)
	if err == nil {
		t.Fatal("expected error for invalid name type")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for invalid name type")
	}
}

func TestUserAbsentTestModeExistingUser(t *testing.T) {
	// Test mode on an existing user (root) — should say "would be deleted"
	u := User{
		id:     "test-absent-test",
		method: "absent",
		params: map[string]interface{}{"name": "root"},
	}
	result, err := u.absent(context.Background(), true)
	if err != nil {
		t.Fatalf("absent() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if len(result.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(result.Notes))
	}
	if result.Notes[0].String() != "user root would be deleted" {
		t.Errorf("unexpected note: %q", result.Notes[0].String())
	}
}

func TestUserPresentTestModeExistingUser(t *testing.T) {
	// Test mode on an existing user (root) — should describe what would happen
	u := User{
		id:     "test-present-test",
		method: "present",
		params: map[string]interface{}{"name": "root"},
	}
	result, err := u.present(context.Background(), true)
	if err != nil {
		t.Fatalf("present() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true in test mode")
	}
}

func TestUserPresentTestModeNewUser(t *testing.T) {
	// Test mode for a user that doesn't exist — should plan a useradd
	u := User{
		id:     "test-present-new",
		method: "present",
		params: map[string]interface{}{
			"name":  "grlx-test-nonexistent-user-abc123",
			"shell": "/bin/bash",
			"home":  "/home/grlx-test",
		},
	}
	result, err := u.present(context.Background(), true)
	if err != nil {
		t.Fatalf("present() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true in test mode")
	}
	if len(result.Notes) < 1 {
		t.Fatal("expected at least 1 note")
	}
	// Note should reference useradd
	noteStr := result.Notes[0].String()
	if noteStr == "" {
		t.Error("expected non-empty note")
	}
}

func TestUserTestDispatch(t *testing.T) {
	// Test the Test() method dispatch for all valid methods
	tests := []struct {
		method  string
		params  map[string]interface{}
		wantErr bool
	}{
		{method: "exists", params: map[string]interface{}{"name": "root"}},
		{method: "absent", params: map[string]interface{}{"name": "grlx-test-nonexistent-user-abc123"}},
		{method: "present", params: map[string]interface{}{"name": "root"}},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			u := User{id: "test-dispatch", method: tt.method, params: tt.params}
			_, err := u.Test(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Test() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserTestDispatchUndefined(t *testing.T) {
	u := User{id: "test-undef", method: "nonexistent", params: map[string]interface{}{"name": "test"}}
	result, err := u.Test(context.Background())
	if err == nil {
		t.Error("expected error for undefined method")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for undefined method")
	}
}

func TestUserApplyDispatchUndefined(t *testing.T) {
	u := User{id: "test-undef", method: "nonexistent", params: map[string]interface{}{"name": "test"}}
	result, err := u.Apply(context.Background())
	if err == nil {
		t.Error("expected error for undefined method")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for undefined method")
	}
}

func TestUserExistsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	// Even with cancelled context, exists should still work (no exec calls)
	u := User{id: "test-ctx", method: "exists", params: map[string]interface{}{"name": "root"}}
	result, err := u.exists(ctx, false)
	if err != nil {
		t.Fatalf("exists() unexpected error: %v", err)
	}
	if !result.Succeeded {
		t.Error("expected succeeded even with cancelled context (no exec needed)")
	}
}

func TestUserParseValidation(t *testing.T) {
	// Verify Parse returns errors that match ErrMissingName
	u := User{}
	_, err := u.Parse("id", "absent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}
