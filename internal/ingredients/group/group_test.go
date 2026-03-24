package group

import (
	"context"
	"errors"
	"fmt"
	"os/user"
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

// stubState tracks calls to the stubbed exec/lookup functions.
type stubState struct {
	calls       []string
	existGroups map[string]*user.Group // groups that "exist"
	failCmds    map[string]error       // command name → error to return
}

func (ss *stubState) record(op string) { ss.calls = append(ss.calls, op) }

// installStubs replaces execCommand, lookupGroup, and groupExistsBy with
// test doubles. Restores originals on cleanup.
func installStubs(t *testing.T) *stubState {
	t.Helper()
	ss := &stubState{
		existGroups: map[string]*user.Group{
			"root": {Name: "root", Gid: "0"},
		},
		failCmds: map[string]error{},
	}

	origExec := execCommand
	origLookup := lookupGroup
	origExists := groupExistsBy

	execCommand = func(_ context.Context, name string, args ...string) error {
		ss.record(name)
		if err, ok := ss.failCmds[name]; ok {
			return err
		}
		// Simulate side effects
		switch name {
		case "groupadd":
			gName := args[len(args)-1]
			gid := "1000" // default
			for i, a := range args {
				if a == "-g" && i+1 < len(args) {
					gid = args[i+1]
				}
			}
			ss.existGroups[gName] = &user.Group{Name: gName, Gid: gid}
		case "groupdel":
			gName := args[len(args)-1]
			delete(ss.existGroups, gName)
		case "groupmod":
			gName := args[len(args)-1]
			if g, ok := ss.existGroups[gName]; ok {
				for i, a := range args {
					if a == "-g" && i+1 < len(args) {
						g.Gid = args[i+1]
					}
				}
			}
		case "gpasswd":
			// no-op for tests
		}
		return nil
	}

	lookupGroup = func(name string) (*user.Group, error) {
		if g, ok := ss.existGroups[name]; ok {
			return g, nil
		}
		return nil, fmt.Errorf("group: unknown group %s", name)
	}

	groupExistsBy = func(name string) bool {
		_, ok := ss.existGroups[name]
		return ok
	}

	t.Cleanup(func() {
		execCommand = origExec
		lookupGroup = origLookup
		groupExistsBy = origExists
	})
	return ss
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

// --- exists ---

func TestGroupExists(t *testing.T) {
	ss := installStubs(t)

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
			_ = ss
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

// --- absent ---

func TestGroupAbsentAlreadyAbsent(t *testing.T) {
	installStubs(t)
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
	installStubs(t)
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
	installStubs(t)
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

func TestGroupAbsentApplySuccess(t *testing.T) {
	ss := installStubs(t)
	ss.existGroups["testgroup"] = &user.Group{Name: "testgroup", Gid: "5000"}

	g := Group{
		id:     "test-absent-apply",
		method: "absent",
		params: map[string]interface{}{"name": "testgroup"},
	}
	result, err := g.absent(context.Background(), false)
	if err != nil {
		t.Fatalf("absent() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	if len(result.Notes) != 1 || result.Notes[0].String() != "group testgroup deleted" {
		t.Errorf("unexpected notes: %v", result.Notes)
	}
	// Verify groupdel was called
	found := false
	for _, c := range ss.calls {
		if c == "groupdel" {
			found = true
		}
	}
	if !found {
		t.Error("expected groupdel to be called")
	}
}

func TestGroupAbsentApplyGroupdelFails(t *testing.T) {
	ss := installStubs(t)
	ss.existGroups["testgroup"] = &user.Group{Name: "testgroup", Gid: "5000"}
	ss.failCmds["groupdel"] = errors.New("permission denied")

	g := Group{
		id:     "test-absent-fail",
		method: "absent",
		params: map[string]interface{}{"name": "testgroup"},
	}
	result, err := g.absent(context.Background(), false)
	if err == nil {
		t.Fatal("expected error from groupdel failure")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result")
	}
}

func TestGroupAbsentApplyGroupStillExists(t *testing.T) {
	ss := installStubs(t)
	ss.existGroups["stubborn"] = &user.Group{Name: "stubborn", Gid: "9999"}

	// Override execCommand to not actually delete the group
	origExec := execCommand
	execCommand = func(_ context.Context, name string, args ...string) error {
		ss.record(name)
		// Don't remove from existGroups — simulates groupdel success but group still exists
		return nil
	}
	t.Cleanup(func() { execCommand = origExec })

	g := Group{
		id:     "test-absent-stubborn",
		method: "absent",
		params: map[string]interface{}{"name": "stubborn"},
	}
	result, err := g.absent(context.Background(), false)
	if err == nil {
		t.Fatal("expected error when group still exists after deletion")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result")
	}
}

// --- present ---

func TestGroupPresentExistingGroupNoChange(t *testing.T) {
	installStubs(t)
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
	installStubs(t)
	g := Group{
		id:     "test-present-new",
		method: "present",
		params: map[string]interface{}{
			"name": "newgroup",
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
	installStubs(t)
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
	installStubs(t)
	g := Group{
		id:     "test-present-members",
		method: "present",
		params: map[string]interface{}{
			"name":    "newgroup",
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
	if len(result.Notes) < 2 {
		t.Errorf("expected at least 2 notes (create + members), got %d", len(result.Notes))
	}
}

func TestGroupPresentTestModeSystemGroup(t *testing.T) {
	installStubs(t)
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
	installStubs(t)
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

func TestGroupPresentApplyCreateNewGroup(t *testing.T) {
	ss := installStubs(t)
	g := Group{
		id:     "test-present-create",
		method: "present",
		params: map[string]interface{}{
			"name": "newgroup",
			"gid":  "5000",
		},
	}
	result, err := g.present(context.Background(), false)
	if err != nil {
		t.Fatalf("present() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if !result.Changed {
		t.Error("expected changed=true for new group")
	}
	// Verify groupadd was called
	found := false
	for _, c := range ss.calls {
		if c == "groupadd" {
			found = true
		}
	}
	if !found {
		t.Error("expected groupadd to be called")
	}
	// Verify group now exists in stubs
	if _, ok := ss.existGroups["newgroup"]; !ok {
		t.Error("expected newgroup to exist after creation")
	}
}

func TestGroupPresentApplyCreateGroupFails(t *testing.T) {
	ss := installStubs(t)
	ss.failCmds["groupadd"] = errors.New("groupadd failed")

	g := Group{
		id:     "test-present-create-fail",
		method: "present",
		params: map[string]interface{}{"name": "failgroup"},
	}
	result, err := g.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error from groupadd failure")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result")
	}
}

func TestGroupPresentApplyCreateWithMembers(t *testing.T) {
	ss := installStubs(t)
	g := Group{
		id:     "test-present-create-members",
		method: "present",
		params: map[string]interface{}{
			"name":    "devteam",
			"members": []interface{}{"alice", "bob"},
		},
	}
	result, err := g.present(context.Background(), false)
	if err != nil {
		t.Fatalf("present() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if !result.Changed {
		t.Error("expected changed=true")
	}
	// Verify both groupadd and gpasswd were called
	gotGroupadd, gotGpasswd := false, false
	for _, c := range ss.calls {
		if c == "groupadd" {
			gotGroupadd = true
		}
		if c == "gpasswd" {
			gotGpasswd = true
		}
	}
	if !gotGroupadd {
		t.Error("expected groupadd to be called")
	}
	if !gotGpasswd {
		t.Error("expected gpasswd to be called for members")
	}
}

func TestGroupPresentApplyCreateWithMembersFails(t *testing.T) {
	ss := installStubs(t)
	ss.failCmds["gpasswd"] = errors.New("gpasswd failed")

	g := Group{
		id:     "test-present-members-fail",
		method: "present",
		params: map[string]interface{}{
			"name":    "devteam",
			"members": []interface{}{"alice"},
		},
	}
	result, err := g.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error from gpasswd failure")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result")
	}
}

func TestGroupPresentApplyModifyGid(t *testing.T) {
	ss := installStubs(t)
	// root group exists with gid=0, modify to gid=9999
	g := Group{
		id:     "test-present-modify-gid",
		method: "present",
		params: map[string]interface{}{
			"name": "root",
			"gid":  "9999",
		},
	}
	result, err := g.present(context.Background(), false)
	if err != nil {
		t.Fatalf("present() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if !result.Changed {
		t.Error("expected changed=true for GID modification")
	}
	// Verify groupmod was called
	found := false
	for _, c := range ss.calls {
		if c == "groupmod" {
			found = true
		}
	}
	if !found {
		t.Error("expected groupmod to be called")
	}
	// Verify GID was updated
	if ss.existGroups["root"].Gid != "9999" {
		t.Errorf("expected root gid=9999, got %s", ss.existGroups["root"].Gid)
	}
}

func TestGroupPresentApplyModifyGidFails(t *testing.T) {
	ss := installStubs(t)
	ss.failCmds["groupmod"] = errors.New("groupmod failed")

	g := Group{
		id:     "test-present-modify-fail",
		method: "present",
		params: map[string]interface{}{
			"name": "root",
			"gid":  "9999",
		},
	}
	result, err := g.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error from groupmod failure")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result")
	}
}

func TestGroupPresentApplyExistingGroupSetMembers(t *testing.T) {
	ss := installStubs(t)
	g := Group{
		id:     "test-present-existing-members",
		method: "present",
		params: map[string]interface{}{
			"name":    "root",
			"members": []interface{}{"alice", "bob"},
		},
	}
	result, err := g.present(context.Background(), false)
	if err != nil {
		t.Fatalf("present() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if !result.Changed {
		t.Error("expected changed=true for member change")
	}
	gotGpasswd := false
	for _, c := range ss.calls {
		if c == "gpasswd" {
			gotGpasswd = true
		}
	}
	if !gotGpasswd {
		t.Error("expected gpasswd to be called for members")
	}
}

func TestGroupPresentApplyExistingGroupSetMembersFails(t *testing.T) {
	ss := installStubs(t)
	ss.failCmds["gpasswd"] = errors.New("gpasswd failed")

	g := Group{
		id:     "test-present-existing-members-fail",
		method: "present",
		params: map[string]interface{}{
			"name":    "root",
			"members": []interface{}{"alice"},
		},
	}
	result, err := g.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error from gpasswd failure")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result")
	}
}

func TestGroupPresentTestModeExistingGroupWithMembers(t *testing.T) {
	installStubs(t)
	g := Group{
		id:     "test-present-test-members",
		method: "present",
		params: map[string]interface{}{
			"name":    "root",
			"members": []interface{}{"alice"},
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
		t.Error("expected changed=true for member change")
	}
}

// --- Test/Apply dispatch ---

func TestGroupTestDispatch(t *testing.T) {
	installStubs(t)
	tests := []struct {
		method  string
		params  map[string]interface{}
		wantErr bool
	}{
		{method: "exists", params: map[string]interface{}{"name": "root"}},
		{method: "absent", params: map[string]interface{}{"name": "nonexistent"}},
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
	installStubs(t)
	g := Group{id: "test-undef", method: "nonexistent", params: map[string]interface{}{"name": "test"}}
	result, err := g.Test(context.Background())
	if err == nil {
		t.Error("expected error for undefined method")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for undefined method")
	}
}

func TestGroupApplyDispatch(t *testing.T) {
	installStubs(t)
	tests := []struct {
		method  string
		params  map[string]interface{}
		wantErr bool
	}{
		{method: "exists", params: map[string]interface{}{"name": "root"}},
		{method: "absent", params: map[string]interface{}{"name": "nonexistent"}},
		{method: "present", params: map[string]interface{}{"name": "root"}},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			g := Group{id: "test-apply", method: tt.method, params: tt.params}
			_, err := g.Apply(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupApplyDispatchUndefined(t *testing.T) {
	installStubs(t)
	g := Group{id: "test-undef", method: "nonexistent", params: map[string]interface{}{"name": "test"}}
	result, err := g.Apply(context.Background())
	if err == nil {
		t.Error("expected error for undefined method")
	}
	if result.Succeeded || !result.Failed {
		t.Error("expected failed result for undefined method")
	}
}

// --- Context cancellation ---

func TestGroupExistsContextCancellation(t *testing.T) {
	installStubs(t)
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

// --- Parse validation ---

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

// --- Helper functions ---

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

func TestStringParam(t *testing.T) {
	params := map[string]interface{}{
		"gid":     "5000",
		"badtype": 42,
	}
	if stringParam(params, "gid") != "5000" {
		t.Errorf("expected '5000', got %q", stringParam(params, "gid"))
	}
	if stringParam(params, "missing") != "" {
		t.Errorf("expected empty string for missing key")
	}
	if stringParam(params, "badtype") != "" {
		t.Errorf("expected empty string for non-string value")
	}
}
