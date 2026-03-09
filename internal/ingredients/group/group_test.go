package group

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

func TestGroupParse(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		method  string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:   "valid absent",
			id:     "test-group-absent",
			method: "absent",
			params: map[string]interface{}{"name": "testgroup"},
		},
		{
			name:   "valid exists",
			id:     "test-group-exists",
			method: "exists",
			params: map[string]interface{}{"name": "testgroup"},
		},
		{
			name:   "valid present with gid",
			id:     "test-group-present",
			method: "present",
			params: map[string]interface{}{
				"name": "testgroup",
				"gid":  "5000",
			},
		},
		{
			name:   "valid present with all options",
			id:     "test-group-present-all",
			method: "present",
			params: map[string]interface{}{
				"name":    "testgroup",
				"gid":     "5000",
				"system":  false,
				"members": []string{"alice", "bob"},
			},
		},
		{
			name:   "valid present with name only",
			id:     "test-group-present-min",
			method: "present",
			params: map[string]interface{}{"name": "testgroup"},
		},
		{
			name:    "missing name in absent",
			id:      "test-group-absent-no-name",
			method:  "absent",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "missing name in exists",
			id:      "test-group-exists-no-name",
			method:  "exists",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "missing name in present",
			id:      "test-group-present-no-name",
			method:  "present",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "empty name string",
			id:      "test-group-empty-name",
			method:  "absent",
			params:  map[string]interface{}{"name": ""},
			wantErr: true,
		},
		{
			name:    "name is not a string",
			id:      "test-group-int-name",
			method:  "absent",
			params:  map[string]interface{}{"name": 42},
			wantErr: true,
		},
		{
			name:    "nil params",
			id:      "test-group-nil-params",
			method:  "absent",
			params:  nil,
			wantErr: true,
		},
		{
			name:    "undefined method",
			id:      "test-group-bad-method",
			method:  "nonexistent",
			params:  map[string]interface{}{"name": "testgroup"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := Group{}
			_, err := g.Parse(tt.id, tt.method, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupMethods(t *testing.T) {
	g := Group{}
	name, methods := g.Methods()
	if name != "group" {
		t.Errorf("expected ingredient name 'group', got %q", name)
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

func TestGroupPropertiesForMethod(t *testing.T) {
	g := Group{}

	tests := []struct {
		method   string
		wantErr  bool
		wantKeys []string
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
			wantKeys: []string{"name", "gid", "system", "members"},
		},
		{
			method:  "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			props, err := g.PropertiesForMethod(tt.method)
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

func TestGroupProperties(t *testing.T) {
	g := Group{
		id:     "test",
		method: "present",
		params: map[string]interface{}{
			"name": "testgroup",
			"gid":  "5000",
		},
	}

	props, err := g.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if props["name"] != "testgroup" {
		t.Errorf("expected name 'testgroup', got %v", props["name"])
	}
	if props["gid"] != "5000" {
		t.Errorf("expected gid '5000', got %v", props["gid"])
	}
}

func TestGroupExists(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected cook.Result
		wantErr  bool
	}{
		{
			name:   "root group exists",
			params: map[string]interface{}{"name": "root"},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     []fmt.Stringer{cook.SimpleNote("group root exists")},
			},
		},
		{
			name:   "nonexistent group",
			params: map[string]interface{}{"name": "grlx-test-nonexistent-group-abc123"},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{cook.SimpleNote("group grlx-test-nonexistent-group-abc123 does not exist")},
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
			g := Group{
				id:     "test-exists",
				method: "exists",
				params: tt.params,
			}
			result, err := g.exists(context.Background(), false)
			if (err != nil) != tt.wantErr {
				t.Errorf("exists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			compareResults(t, result, tt.expected)
		})
	}
}

func TestGroupAbsentAlreadyAbsent(t *testing.T) {
	g := Group{
		id:     "test-absent",
		method: "absent",
		params: map[string]interface{}{"name": "grlx-test-nonexistent-group-abc123"},
	}
	result, err := g.absent(context.Background(), false)
	if err != nil {
		t.Fatalf("absent() unexpected error: %v", err)
	}
	expected := cook.Result{
		Succeeded: true,
		Failed:    false,
		Notes:     []fmt.Stringer{cook.SimpleNote("group grlx-test-nonexistent-group-abc123 already absent, nothing to do")},
	}
	compareResults(t, result, expected)
}

func TestGroupAbsentInvalidName(t *testing.T) {
	g := Group{
		id:     "test-absent-invalid",
		method: "absent",
		params: map[string]interface{}{"name": 42},
	}
	result, err := g.absent(context.Background(), false)
	if err == nil {
		t.Fatal("expected error for invalid name type")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for invalid name type")
	}
}

func TestGroupAbsentTestModeExistingGroup(t *testing.T) {
	g := Group{
		id:     "test-absent-test",
		method: "absent",
		params: map[string]interface{}{"name": "root"},
	}
	result, err := g.absent(context.Background(), true)
	if err != nil {
		t.Fatalf("absent() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true in test mode")
	}
	if len(result.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(result.Notes))
	}
	if result.Notes[0].String() != "group root would be deleted" {
		t.Errorf("unexpected note: %q", result.Notes[0].String())
	}
}

func TestGroupPresentExistingGroupNoChange(t *testing.T) {
	g := Group{
		id:     "test-present-existing",
		method: "present",
		params: map[string]interface{}{"name": "root"},
	}
	result, err := g.present(context.Background(), false)
	if err != nil {
		t.Fatalf("present() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if result.Changed {
		t.Error("expected no changes for existing group with matching state")
	}
	if len(result.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(result.Notes))
	}
	if result.Notes[0].String() != "group already exists" {
		t.Errorf("unexpected note: %q", result.Notes[0].String())
	}
}

func TestGroupPresentTestModeNewGroup(t *testing.T) {
	g := Group{
		id:     "test-present-new",
		method: "present",
		params: map[string]interface{}{
			"name": "grlx-test-nonexistent-group-abc123",
			"gid":  "9999",
		},
	}
	result, err := g.present(context.Background(), true)
	if err != nil {
		t.Fatalf("present() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true in test mode for new group")
	}
	if len(result.Notes) < 1 {
		t.Fatal("expected at least 1 note")
	}
}

func TestGroupPresentTestModeExistingGroupGidChange(t *testing.T) {
	g := Group{
		id:     "test-present-mod",
		method: "present",
		params: map[string]interface{}{
			"name": "root",
			"gid":  "99999",
		},
	}
	result, err := g.present(context.Background(), true)
	if err != nil {
		t.Fatalf("present() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true in test mode for GID change")
	}
}

func TestGroupPresentTestModeNewGroupWithMembers(t *testing.T) {
	g := Group{
		id:     "test-present-members",
		method: "present",
		params: map[string]interface{}{
			"name":    "grlx-test-nonexistent-group-abc123",
			"members": []interface{}{"alice", "bob"},
		},
	}
	result, err := g.present(context.Background(), true)
	if err != nil {
		t.Fatalf("present() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	// Should have notes about both creation and members
	if len(result.Notes) < 2 {
		t.Errorf("expected at least 2 notes (create + members), got %d", len(result.Notes))
	}
}

func TestGroupPresentTestModeSystemGroup(t *testing.T) {
	g := Group{
		id:     "test-present-system",
		method: "present",
		params: map[string]interface{}{
			"name":   "grlx-test-sysgroup",
			"system": true,
		},
	}
	result, err := g.present(context.Background(), true)
	if err != nil {
		t.Fatalf("present() test mode unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result in test mode")
	}
	if !result.Changed {
		t.Error("expected changed=true for new system group")
	}
}

func TestGroupPresentInvalidName(t *testing.T) {
	g := Group{
		id:     "test-present-invalid",
		method: "present",
		params: map[string]interface{}{"name": ""},
	}
	result, err := g.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for empty name")
	}
}

func TestGroupTestDispatch(t *testing.T) {
	tests := []struct {
		method  string
		params  map[string]interface{}
		wantErr bool
	}{
		{method: "exists", params: map[string]interface{}{"name": "root"}},
		{method: "absent", params: map[string]interface{}{"name": "grlx-test-nonexistent-group-abc123"}},
		{method: "present", params: map[string]interface{}{"name": "root"}},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			g := Group{id: "test-dispatch", method: tt.method, params: tt.params}
			_, err := g.Test(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Test() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupTestDispatchUndefined(t *testing.T) {
	g := Group{id: "test-undef", method: "nonexistent", params: map[string]interface{}{"name": "test"}}
	result, err := g.Test(context.Background())
	if err == nil {
		t.Error("expected error for undefined method")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for undefined method")
	}
}

func TestGroupApplyDispatchUndefined(t *testing.T) {
	g := Group{id: "test-undef", method: "nonexistent", params: map[string]interface{}{"name": "test"}}
	result, err := g.Apply(context.Background())
	if err == nil {
		t.Error("expected error for undefined method")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for undefined method")
	}
}

func TestGroupExistsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	g := Group{id: "test-ctx", method: "exists", params: map[string]interface{}{"name": "root"}}
	result, err := g.exists(ctx, false)
	if err != nil {
		t.Fatalf("exists() unexpected error: %v", err)
	}
	if !result.Succeeded {
		t.Error("expected succeeded even with cancelled context (no exec needed)")
	}
}

func TestGroupParseValidation(t *testing.T) {
	g := Group{}
	_, err := g.Parse("id", "absent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestBuildGroupaddArgs(t *testing.T) {
	args := buildGroupaddArgs("mygroup", "5000", true)
	expected := []string{"-g", "5000", "-r", "mygroup"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestBuildGroupaddArgsMinimal(t *testing.T) {
	args := buildGroupaddArgs("mygroup", "", false)
	expected := []string{"mygroup"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	if args[0] != "mygroup" {
		t.Errorf("expected 'mygroup', got %q", args[0])
	}
}

func TestBuildGroupmodArgs(t *testing.T) {
	args := buildGroupmodArgs("mygroup", "6000")
	expected := []string{"-g", "6000", "mygroup"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestStringSliceParam(t *testing.T) {
	// Test []string (direct)
	params := map[string]interface{}{
		"members": []string{"alice", "bob"},
	}
	got := stringSliceParam(params, "members")
	if len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Errorf("expected [alice bob], got %v", got)
	}

	// Test []interface{} (from JSON)
	params2 := map[string]interface{}{
		"members": []interface{}{"alice", "bob"},
	}
	got2 := stringSliceParam(params2, "members")
	if len(got2) != 2 || got2[0] != "alice" || got2[1] != "bob" {
		t.Errorf("expected [alice bob], got %v", got2)
	}

	// Test missing key
	got3 := stringSliceParam(params, "missing")
	if got3 != nil {
		t.Errorf("expected nil, got %v", got3)
	}
}

func TestBoolParam(t *testing.T) {
	params := map[string]interface{}{
		"system":  true,
		"badtype": "yes",
	}
	if !boolParam(params, "system", false) {
		t.Error("expected true for system")
	}
	if !boolParam(params, "missing", true) {
		t.Error("expected default true for missing key")
	}
	if boolParam(params, "badtype", false) {
		t.Error("expected default false for bad type")
	}
}
