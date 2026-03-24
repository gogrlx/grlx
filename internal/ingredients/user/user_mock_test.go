package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"testing"
)

// mockCmd wraps exec.CommandContext but runs "true" or "false" to simulate
// success or failure, while recording the intended command for assertions.
type cmdRecorder struct {
	calls [][]string
}

func (r *cmdRecorder) successFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	r.calls = append(r.calls, append([]string{name}, args...))
	return exec.CommandContext(ctx, "true")
}

func (r *cmdRecorder) failFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	r.calls = append(r.calls, append([]string{name}, args...))
	return exec.CommandContext(ctx, "false")
}

// withMockExec temporarily overrides execCommandContext and restores it after.
func withMockExec(factory func(context.Context, string, ...string) *exec.Cmd, fn func()) {
	orig := execCommandContext
	execCommandContext = factory
	defer func() { execCommandContext = orig }()
	fn()
}

// withMockLookup temporarily overrides lookupUser and restores it after.
func withMockLookup(mock func(string) (*user.User, error), fn func()) {
	orig := lookupUser
	lookupUser = mock
	defer func() { lookupUser = orig }()
	fn()
}

// lookupNotFound returns a mock that always fails (user not found).
func lookupNotFound(_ string) (*user.User, error) {
	return nil, errors.New("user not found")
}

// lookupFound returns a mock that always succeeds with the given user.
func lookupFound(u *user.User) func(string) (*user.User, error) {
	return func(_ string) (*user.User, error) {
		return u, nil
	}
}

// --- Apply dispatch tests ---

func TestApplyDispatchPresent(t *testing.T) {
	rec := &cmdRecorder{}
	withMockExec(rec.successFactory, func() {
		withMockLookup(lookupNotFound, func() {
			u := User{id: "apply-present", method: "present", params: map[string]interface{}{"name": "newuser"}}
			result, err := u.Apply(context.Background())
			if err != nil {
				t.Fatalf("Apply() unexpected error: %v", err)
			}
			if !result.Succeeded || result.Failed {
				t.Error("expected succeeded result")
			}
			if !result.Changed {
				t.Error("expected changed=true for new user creation")
			}
		})
	})
}

func TestApplyDispatchExists(t *testing.T) {
	withMockLookup(lookupFound(&user.User{Username: "root"}), func() {
		u := User{id: "apply-exists", method: "exists", params: map[string]interface{}{"name": "root"}}
		result, err := u.Apply(context.Background())
		if err != nil {
			t.Fatalf("Apply() unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Error("expected succeeded result")
		}
	})
}

func TestApplyDispatchAbsent(t *testing.T) {
	withMockLookup(lookupNotFound, func() {
		u := User{id: "apply-absent", method: "absent", params: map[string]interface{}{"name": "nouser"}}
		result, err := u.Apply(context.Background())
		if err != nil {
			t.Fatalf("Apply() unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Error("expected succeeded for already-absent user")
		}
	})
}

// --- present: useradd success ---

func TestPresentApplyNewUser(t *testing.T) {
	rec := &cmdRecorder{}
	withMockExec(rec.successFactory, func() {
		withMockLookup(lookupNotFound, func() {
			u := User{
				id: "present-new", method: "present",
				params: map[string]interface{}{
					"name":       "testuser",
					"uid":        "5000",
					"shell":      "/bin/zsh",
					"home":       "/home/testuser",
					"createhome": true,
				},
			}
			result, err := u.present(context.Background(), false)
			if err != nil {
				t.Fatalf("present() error: %v", err)
			}
			if !result.Succeeded || result.Failed || !result.Changed {
				t.Error("expected succeeded+changed for new user")
			}
			if len(rec.calls) != 1 {
				t.Fatalf("expected 1 exec call, got %d", len(rec.calls))
			}
			if rec.calls[0][0] != "useradd" {
				t.Errorf("expected useradd, got %s", rec.calls[0][0])
			}
		})
	})
}

// --- present: useradd failure ---

func TestPresentApplyNewUserFailure(t *testing.T) {
	rec := &cmdRecorder{}
	withMockExec(rec.failFactory, func() {
		withMockLookup(lookupNotFound, func() {
			u := User{
				id: "present-fail", method: "present",
				params: map[string]interface{}{"name": "testuser"},
			}
			result, err := u.present(context.Background(), false)
			if err == nil {
				t.Fatal("expected error for useradd failure")
			}
			if !result.Failed || result.Succeeded {
				t.Error("expected failed result")
			}
			if len(result.Notes) < 1 {
				t.Fatal("expected failure note")
			}
			if !strings.Contains(result.Notes[0].String(), "failed to create user") {
				t.Errorf("unexpected note: %s", result.Notes[0].String())
			}
		})
	})
}

// --- present: usermod success ---

func TestPresentApplyExistingUserWithChanges(t *testing.T) {
	rec := &cmdRecorder{}
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/olduser",
		Name:    "Old Name",
	}
	withMockExec(rec.successFactory, func() {
		withMockLookup(lookupFound(existing), func() {
			u := User{
				id: "present-mod", method: "present",
				params: map[string]interface{}{
					"name":    "testuser",
					"uid":     "2000",
					"home":    "/home/newuser",
					"comment": "New Name",
				},
			}
			result, err := u.present(context.Background(), false)
			if err != nil {
				t.Fatalf("present() error: %v", err)
			}
			if !result.Succeeded || result.Failed || !result.Changed {
				t.Error("expected succeeded+changed for modified user")
			}
			if len(rec.calls) != 1 {
				t.Fatalf("expected 1 exec call, got %d", len(rec.calls))
			}
			if rec.calls[0][0] != "usermod" {
				t.Errorf("expected usermod, got %s", rec.calls[0][0])
			}
		})
	})
}

// --- present: usermod failure ---

func TestPresentApplyExistingUserModFailure(t *testing.T) {
	rec := &cmdRecorder{}
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/olduser",
		Name:    "Old Name",
	}
	withMockExec(rec.failFactory, func() {
		withMockLookup(lookupFound(existing), func() {
			u := User{
				id: "present-mod-fail", method: "present",
				params: map[string]interface{}{
					"name": "testuser",
					"uid":  "2000",
				},
			}
			result, err := u.present(context.Background(), false)
			if err == nil {
				t.Fatal("expected error for usermod failure")
			}
			if !result.Failed || result.Succeeded {
				t.Error("expected failed result")
			}
			if len(result.Notes) < 1 || !strings.Contains(result.Notes[0].String(), "failed to modify user") {
				t.Errorf("expected failure note, got %v", result.Notes)
			}
		})
	})
}

// --- present: test mode with existing user needing changes ---

func TestPresentTestModeExistingUserWithChanges(t *testing.T) {
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/olduser",
		Name:    "Old Name",
	}
	withMockLookup(lookupFound(existing), func() {
		u := User{
			id: "present-test-mod", method: "present",
			params: map[string]interface{}{
				"name":    "testuser",
				"uid":     "2000",
				"comment": "New Name",
			},
		}
		result, err := u.present(context.Background(), true)
		if err != nil {
			t.Fatalf("present() test mode error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Error("expected succeeded result")
		}
		if !result.Changed {
			t.Error("expected changed=true in test mode")
		}
		if len(result.Notes) < 1 || !strings.Contains(result.Notes[0].String(), "would modify user") {
			t.Errorf("expected 'would modify' note, got %v", result.Notes)
		}
	})
}

// --- absent: actual deletion success ---

func TestAbsentApplyExistingUser(t *testing.T) {
	callCount := 0
	rec := &cmdRecorder{}
	// First lookup: user exists. Second lookup (post-delete check): user gone.
	withMockExec(rec.successFactory, func() {
		withMockLookup(func(name string) (*user.User, error) {
			callCount++
			if callCount == 1 {
				return &user.User{Username: name}, nil
			}
			return nil, errors.New("not found")
		}, func() {
			u := User{
				id: "absent-del", method: "absent",
				params: map[string]interface{}{"name": "testuser"},
			}
			result, err := u.absent(context.Background(), false)
			if err != nil {
				t.Fatalf("absent() error: %v", err)
			}
			if !result.Succeeded || result.Failed || !result.Changed {
				t.Error("expected succeeded+changed for deleted user")
			}
			if len(rec.calls) != 1 || rec.calls[0][0] != "userdel" {
				t.Errorf("expected userdel call, got %v", rec.calls)
			}
			// Should NOT have -r flag (no purge)
			for _, arg := range rec.calls[0] {
				if arg == "-r" {
					t.Error("did not expect -r flag without purge")
				}
			}
		})
	})
}

// --- absent: with purge ---

func TestAbsentApplyWithPurge(t *testing.T) {
	callCount := 0
	rec := &cmdRecorder{}
	withMockExec(rec.successFactory, func() {
		withMockLookup(func(name string) (*user.User, error) {
			callCount++
			if callCount == 1 {
				return &user.User{Username: name}, nil
			}
			return nil, errors.New("not found")
		}, func() {
			u := User{
				id: "absent-purge", method: "absent",
				params: map[string]interface{}{"name": "testuser", "purge": true},
			}
			result, err := u.absent(context.Background(), false)
			if err != nil {
				t.Fatalf("absent() error: %v", err)
			}
			if !result.Succeeded || !result.Changed {
				t.Error("expected succeeded+changed")
			}
			// Should have -r flag
			found := false
			for _, arg := range rec.calls[0] {
				if arg == "-r" {
					found = true
				}
			}
			if !found {
				t.Errorf("expected -r flag with purge, got %v", rec.calls[0])
			}
		})
	})
}

// --- absent: userdel failure ---

func TestAbsentApplyDeleteFailure(t *testing.T) {
	rec := &cmdRecorder{}
	withMockExec(rec.failFactory, func() {
		withMockLookup(lookupFound(&user.User{Username: "testuser"}), func() {
			u := User{
				id: "absent-fail", method: "absent",
				params: map[string]interface{}{"name": "testuser"},
			}
			result, err := u.absent(context.Background(), false)
			if err == nil {
				t.Fatal("expected error for userdel failure")
			}
			if !result.Failed {
				t.Error("expected failed result")
			}
			if len(result.Notes) < 1 || !strings.Contains(result.Notes[0].String(), "failed to delete") {
				t.Errorf("unexpected notes: %v", result.Notes)
			}
		})
	})
}

// --- absent: user still exists after delete ---

func TestAbsentApplyUserStillExistsAfterDelete(t *testing.T) {
	rec := &cmdRecorder{}
	// userdel "succeeds" but user still exists
	withMockExec(rec.successFactory, func() {
		withMockLookup(lookupFound(&user.User{Username: "testuser"}), func() {
			u := User{
				id: "absent-stuck", method: "absent",
				params: map[string]interface{}{"name": "testuser"},
			}
			result, err := u.absent(context.Background(), false)
			if err == nil {
				t.Fatal("expected error when user persists after delete")
			}
			if !result.Failed {
				t.Error("expected failed result")
			}
		})
	})
}

// --- present: empty name ---

func TestPresentEmptyName(t *testing.T) {
	u := User{
		id: "present-empty", method: "present",
		params: map[string]interface{}{"name": ""},
	}
	result, err := u.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !result.Failed {
		t.Error("expected failed result")
	}
}

// --- absent: empty name ---

func TestAbsentEmptyName(t *testing.T) {
	u := User{
		id: "absent-empty", method: "absent",
		params: map[string]interface{}{"name": ""},
	}
	result, err := u.absent(context.Background(), false)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !result.Failed {
		t.Error("expected failed result")
	}
}

// --- buildUsermodArgs: shell always set if specified ---

func TestBuildUsermodArgsShellAlwaysSet(t *testing.T) {
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/testuser",
		Name:    "Test User",
	}
	// Shell is always set when specified (can't compare with existing)
	args := buildUsermodArgs("testuser", "", "", "/bin/zsh", "", "", "", nil, existing)
	if args == nil {
		t.Fatal("expected non-nil args when shell is specified")
	}
	found := false
	for i, a := range args {
		if a == "-s" && i+1 < len(args) && args[i+1] == "/bin/zsh" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected -s /bin/zsh in args, got %v", args)
	}
}

// --- buildUsermodArgs: groups always set if specified ---

func TestBuildUsermodArgsGroupsAlwaysSet(t *testing.T) {
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/testuser",
		Name:    "Test User",
	}
	args := buildUsermodArgs("testuser", "", "", "", "", "", "", []string{"wheel", "docker"}, existing)
	if args == nil {
		t.Fatal("expected non-nil args when groups specified")
	}
	found := false
	for i, a := range args {
		if a == "-G" && i+1 < len(args) && args[i+1] == "wheel,docker" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected -G wheel,docker in args, got %v", args)
	}
}

// --- buildUsermodArgs: gid change ---

func TestBuildUsermodArgsGidChange(t *testing.T) {
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/testuser",
		Name:    "Test User",
	}
	args := buildUsermodArgs("testuser", "", "2000", "", "", "", "", nil, existing)
	if args == nil {
		t.Fatal("expected non-nil args when gid differs")
	}
	found := false
	for i, a := range args {
		if a == "-g" && i+1 < len(args) && args[i+1] == "2000" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected -g 2000 in args, got %v", args)
	}
}

// --- stringSliceParam: non-string items in []interface{} ---

func TestStringSliceParamNonStringItems(t *testing.T) {
	params := map[string]interface{}{
		"groups": []interface{}{"valid", 42, "also-valid"},
	}
	got := stringSliceParam(params, "groups")
	// Should only include the string items
	if len(got) != 2 || got[0] != "valid" || got[1] != "also-valid" {
		t.Errorf("expected [valid also-valid], got %v", got)
	}
}

// --- stringSliceParam: wrong type entirely ---

func TestStringSliceParamWrongType(t *testing.T) {
	params := map[string]interface{}{
		"groups": 42,
	}
	got := stringSliceParam(params, "groups")
	if got != nil {
		t.Errorf("expected nil for wrong type, got %v", got)
	}
}

// --- shadowPasswordHash: malformed line ---

func TestShadowPasswordHashMalformedLine(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "shadow-malformed-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Line with only username and no colon-separated hash field
	content := "testuser\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	tmpFile.Close()

	origPath := shadowFilePath
	shadowFilePath = tmpFile.Name()
	defer func() { shadowFilePath = origPath }()

	hash, err := shadowPasswordHash("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Malformed line shouldn't match the prefix "testuser:" check
	if hash != "" {
		t.Errorf("expected empty hash for malformed line, got %q", hash)
	}
}

// --- present: password hash validation on apply ---

func TestPresentApplyInvalidPasswordHash(t *testing.T) {
	u := User{
		id: "present-bad-hash-apply", method: "present",
		params: map[string]interface{}{
			"name":          "testuser",
			"password_hash": "not-a-hash",
		},
	}
	result, err := u.present(context.Background(), false)
	if err == nil {
		t.Fatal("expected error for invalid password hash")
	}
	if !result.Failed {
		t.Error("expected failed result")
	}
}

// --- present: existing user with password hash change ---

func TestPresentApplyExistingUserPasswordHashChange(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "shadow-pwchange-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "testuser:$6$oldsalt$oldhash:19000:0:99999:7:::\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	tmpFile.Close()

	origShadow := shadowFilePath
	shadowFilePath = tmpFile.Name()
	defer func() { shadowFilePath = origShadow }()

	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/testuser",
		Name:    "Test User",
	}

	rec := &cmdRecorder{}
	withMockExec(rec.successFactory, func() {
		withMockLookup(lookupFound(existing), func() {
			u := User{
				id: "present-pw-change", method: "present",
				params: map[string]interface{}{
					"name":          "testuser",
					"password_hash": "$6$newsalt$newhash",
				},
			}
			result, err := u.present(context.Background(), false)
			if err != nil {
				t.Fatalf("present() error: %v", err)
			}
			if !result.Succeeded || !result.Changed {
				t.Error("expected succeeded+changed for password hash change")
			}
			// Verify -p was passed
			if len(rec.calls) < 1 {
				t.Fatal("expected exec call")
			}
			cmd := strings.Join(rec.calls[0], " ")
			if !strings.Contains(cmd, "-p") {
				t.Errorf("expected -p in usermod args, got %s", cmd)
			}
		})
	})
}

// --- present: existing user with all properties matching (no-op with shell) ---

func TestPresentExistingUserNoChangeExceptShellUnknown(t *testing.T) {
	// When shell is specified, usermod is always called because we can't
	// compare it with the existing user (os/user.User doesn't expose shell).
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/testuser",
		Name:    "Test User",
	}
	rec := &cmdRecorder{}
	withMockExec(rec.successFactory, func() {
		withMockLookup(lookupFound(existing), func() {
			u := User{
				id: "present-shell-always", method: "present",
				params: map[string]interface{}{
					"name":  "testuser",
					"uid":   "1000",
					"gid":   "1000",
					"home":  "/home/testuser",
					"shell": "/bin/bash",
				},
			}
			result, err := u.present(context.Background(), false)
			if err != nil {
				t.Fatalf("present() error: %v", err)
			}
			if !result.Succeeded {
				t.Error("expected succeeded")
			}
			// Shell causes usermod even if other fields match
			if !result.Changed {
				t.Error("expected changed=true because shell always triggers usermod")
			}
		})
	})
}

// --- buildUseraddArgs: minimal (name only, defaults) ---

func TestBuildUseraddArgsMinimal(t *testing.T) {
	args := buildUseraddArgs("minuser", "", "", "", "", "", "", nil, true, false)
	expected := []string{"-m", "minuser"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], a)
		}
	}
}

// --- buildUseraddArgs: no createhome ---

func TestBuildUseraddArgsNoCreateHome(t *testing.T) {
	args := buildUseraddArgs("noHomeUser", "", "", "", "", "", "", nil, false, false)
	expected := []string{"noHomeUser"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for _, a := range args {
		if a == "-m" {
			t.Error("did not expect -m flag when createhome=false")
		}
	}
}

// --- Properties: handles all param types ---

func TestPropertiesRoundTrip(t *testing.T) {
	u := User{
		id: "test-props", method: "present",
		params: map[string]interface{}{
			"name":       "testuser",
			"uid":        "1000",
			"groups":     []interface{}{"wheel", "docker"},
			"createhome": true,
			"system":     false,
		},
	}
	props, err := u.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if props["name"] != "testuser" {
		t.Errorf("expected name 'testuser', got %v", props["name"])
	}
	if fmt.Sprintf("%v", props["createhome"]) != "true" {
		t.Errorf("expected createhome true, got %v", props["createhome"])
	}
}

// --- validate: non-"name" required field missing (currently only "name" is required, but test the path) ---

func TestValidateRequiredField(t *testing.T) {
	u := User{}
	// All current methods only require "name", which is tested.
	// Test with a valid parse to exercise the full validation loop.
	_, err := u.Parse("test-val", "present", map[string]interface{}{
		"name":  "testuser",
		"uid":   "1000",
		"shell": "/bin/bash",
	})
	if err != nil {
		t.Errorf("Parse() unexpected error: %v", err)
	}
}

// --- Test dispatch through Test method for test mode ---

func TestTestDispatchAllMethods(t *testing.T) {
	tests := []struct {
		method string
		params map[string]interface{}
	}{
		{"exists", map[string]interface{}{"name": "root"}},
		{"absent", map[string]interface{}{"name": "nonexistent-user-xyz"}},
		{"present", map[string]interface{}{"name": "root"}},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			u := User{id: "test-all", method: tt.method, params: tt.params}
			result, err := u.Test(context.Background())
			if err != nil {
				t.Errorf("Test() error: %v", err)
			}
			if !result.Succeeded {
				t.Error("expected succeeded")
			}
		})
	}
}

// --- shadowPasswordHash: line with username prefix but only one field ---

func TestShadowPasswordHashSingleField(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "shadow-single-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// A line that starts with "testuser:" but has no further colon — impossible
	// in practice since SplitN("testuser:", ":", 3) = ["testuser", ""], but
	// we exercise the path anyway by using just "testuser:" with nothing after.
	content := "testuser:\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	tmpFile.Close()

	origPath := shadowFilePath
	shadowFilePath = tmpFile.Name()
	defer func() { shadowFilePath = origPath }()

	hash, err := shadowPasswordHash("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "testuser:" splits to ["testuser", ""], so hash should be empty string
	if hash != "" {
		t.Errorf("expected empty hash, got %q", hash)
	}
}

// --- absent: test mode for nonexistent user (already absent) ---

func TestAbsentTestModeNonexistentUser(t *testing.T) {
	withMockLookup(lookupNotFound, func() {
		u := User{
			id: "absent-test-nouser", method: "absent",
			params: map[string]interface{}{"name": "nouser"},
		}
		result, err := u.absent(context.Background(), true)
		if err != nil {
			t.Fatalf("absent() error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Error("expected succeeded for already-absent user")
		}
		if result.Changed {
			t.Error("expected changed=false for already-absent user")
		}
	})
}

// --- absent: test mode for existing user ---

func TestAbsentTestModeExistingUserMocked(t *testing.T) {
	withMockLookup(lookupFound(&user.User{Username: "testuser"}), func() {
		u := User{
			id: "absent-test-exists", method: "absent",
			params: map[string]interface{}{"name": "testuser"},
		}
		result, err := u.absent(context.Background(), true)
		if err != nil {
			t.Fatalf("absent() error: %v", err)
		}
		if !result.Succeeded || !result.Changed {
			t.Error("expected succeeded+changed in test mode")
		}
	})
}
