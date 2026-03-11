package user

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"
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
			name:   "valid absent with purge",
			id:     "test-user-absent-purge",
			method: "absent",
			params: map[string]interface{}{"name": "testuser", "purge": true},
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
				"name":       "testuser",
				"uid":        "1234",
				"gid":        "1234",
				"shell":      "/bin/bash",
				"home":       "/home/testuser",
				"groups":     []string{"wheel", "docker"},
				"comment":    "Test User",
				"createhome": true,
				"system":     false,
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
		method   string
		wantErr  bool
		wantKeys []string
	}{
		{
			method:   "absent",
			wantKeys: []string{"name", "purge"},
		},
		{
			method:   "exists",
			wantKeys: []string{"name"},
		},
		{
			method:   "present",
			wantKeys: []string{"name", "uid", "gid", "groups", "shell", "home", "comment", "createhome", "system", "password_hash"},
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
	if !result.Changed {
		t.Error("expected changed=true in test mode")
	}
	if len(result.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(result.Notes))
	}
	if result.Notes[0].String() != "user root would be deleted" {
		t.Errorf("unexpected note: %q", result.Notes[0].String())
	}
}

func TestUserPresentTestModeExistingUser(t *testing.T) {
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
}

func TestUserPresentTestModeNewUser(t *testing.T) {
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
	noteStr := result.Notes[0].String()
	if noteStr == "" {
		t.Error("expected non-empty note")
	}
}

func TestUserPresentExistingNoChanges(t *testing.T) {
	// root user with no specific properties — should detect no changes needed
	u := User{
		id:     "test-present-noop",
		method: "present",
		params: map[string]interface{}{"name": "root"},
	}
	result, err := u.present(context.Background(), false)
	if err != nil {
		t.Fatalf("present() unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("expected succeeded result")
	}
	if result.Changed {
		t.Error("expected changed=false when no modifications needed")
	}
}

func TestUserPresentTestModeWithAllOptions(t *testing.T) {
	u := User{
		id:     "test-present-all",
		method: "present",
		params: map[string]interface{}{
			"name":       "grlx-test-nonexistent-user-abc123",
			"uid":        "9999",
			"gid":        "9999",
			"shell":      "/bin/zsh",
			"home":       "/home/grlx-test",
			"comment":    "Test User",
			"groups":     []interface{}{"wheel", "docker"},
			"createhome": true,
			"system":     false,
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
}

func TestUserPresentTestModeSystemUser(t *testing.T) {
	u := User{
		id:     "test-present-system",
		method: "present",
		params: map[string]interface{}{
			"name":       "grlx-test-svc",
			"system":     true,
			"createhome": false,
			"shell":      "/usr/sbin/nologin",
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
		t.Error("expected changed=true for new system user")
	}
}

func TestUserTestDispatch(t *testing.T) {
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
	u := User{}
	_, err := u.Parse("id", "absent", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestStringSliceParam(t *testing.T) {
	// Test []string (direct)
	params := map[string]interface{}{
		"groups": []string{"wheel", "docker"},
	}
	got := stringSliceParam(params, "groups")
	if len(got) != 2 || got[0] != "wheel" || got[1] != "docker" {
		t.Errorf("expected [wheel docker], got %v", got)
	}

	// Test []interface{} (from JSON)
	params2 := map[string]interface{}{
		"groups": []interface{}{"wheel", "docker"},
	}
	got2 := stringSliceParam(params2, "groups")
	if len(got2) != 2 || got2[0] != "wheel" || got2[1] != "docker" {
		t.Errorf("expected [wheel docker], got %v", got2)
	}

	// Test missing key
	got3 := stringSliceParam(params, "missing")
	if got3 != nil {
		t.Errorf("expected nil, got %v", got3)
	}
}

func TestBoolParam(t *testing.T) {
	params := map[string]interface{}{
		"createhome": true,
		"system":     false,
		"badtype":    "yes",
	}
	if !boolParam(params, "createhome", false) {
		t.Error("expected true for createhome")
	}
	if boolParam(params, "system", true) {
		t.Error("expected false for system")
	}
	if !boolParam(params, "missing", true) {
		t.Error("expected default true for missing key")
	}
	if boolParam(params, "badtype", false) {
		t.Error("expected default false for bad type")
	}
}

func TestBuildUseraddArgs(t *testing.T) {
	args := buildUseraddArgs("testuser", "1000", "1000", "/bin/bash", "/home/test", "Test User", "", []string{"wheel", "docker"}, true, false)
	expected := []string{"-u", "1000", "-g", "1000", "-s", "/bin/bash", "-d", "/home/test", "-c", "Test User", "-G", "wheel,docker", "-m", "testuser"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestBuildUseraddArgsSystem(t *testing.T) {
	args := buildUseraddArgs("svcuser", "", "", "/usr/sbin/nologin", "", "", "", nil, false, true)
	expected := []string{"-s", "/usr/sbin/nologin", "-r", "svcuser"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestBuildUseraddArgsWithPasswordHash(t *testing.T) {
	hash := "$6$rounds=5000$salt$hashvalue"
	args := buildUseraddArgs("testuser", "", "", "", "", "", hash, nil, true, false)
	expected := []string{"-p", hash, "-m", "testuser"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg %d: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestBuildUsermodArgsNoChanges(t *testing.T) {
	existing := &user.User{
		Uid:     "0",
		Gid:     "0",
		HomeDir: "/root",
		Name:    "root",
	}
	args := buildUsermodArgs("root", "0", "0", "", "/root", "root", "", nil, existing)
	if args != nil {
		t.Errorf("expected nil args when no changes needed, got %v", args)
	}
}

func TestBuildUsermodArgsWithChanges(t *testing.T) {
	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/olduser",
		Name:    "Old Name",
	}
	args := buildUsermodArgs("testuser", "1001", "", "", "/home/newuser", "New Name", "", []string{"wheel"}, existing)
	// Should have: -u 1001 -d /home/newuser -c "New Name" -G wheel testuser
	if args == nil {
		t.Fatal("expected non-nil args")
	}
	// Verify the name is last
	if args[len(args)-1] != "testuser" {
		t.Errorf("expected username last, got %q", args[len(args)-1])
	}
}

func TestIsValidPasswordHash(t *testing.T) {
	tests := []struct {
		hash  string
		valid bool
	}{
		{"$6$rounds=5000$salt$hashvalue", true},
		{"$5$salt$hashvalue", true},
		{"$1$salt$hashvalue", true},
		{"$y$j9T$salt$hashvalue", true},
		{"$2b$12$salt.hashvalue", true},
		{"$2a$12$salt.hashvalue", true},
		{"plaintext", false},
		{"", false},
		{"$7$unknown", false},
		{"notahash$6$", false},
	}
	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			if got := isValidPasswordHash(tt.hash); got != tt.valid {
				t.Errorf("isValidPasswordHash(%q) = %v, want %v", tt.hash, got, tt.valid)
			}
		})
	}
}

func TestShadowPasswordHash(t *testing.T) {
	// Create a temporary shadow file for testing.
	tmpFile, err := os.CreateTemp("", "shadow-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `root:$6$salt$rootHash:19000:0:99999:7:::
daemon:*:19000:0:99999:7:::
testuser:$6$rounds=5000$testsalt$testhashvalue:19000:0:99999:7:::
nobody:!:19000:0:99999:7:::
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp shadow: %v", err)
	}
	tmpFile.Close()

	// Override the shadow file path for testing.
	origPath := shadowFilePath
	shadowFilePath = tmpFile.Name()
	defer func() { shadowFilePath = origPath }()

	tests := []struct {
		username string
		wantHash string
		wantErr  bool
	}{
		{"root", "$6$salt$rootHash", false},
		{"testuser", "$6$rounds=5000$testsalt$testhashvalue", false},
		{"daemon", "*", false},
		{"nobody", "!", false},
		{"nonexistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			hash, err := shadowPasswordHash(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("shadowPasswordHash(%q) error = %v, wantErr %v", tt.username, err, tt.wantErr)
				return
			}
			if hash != tt.wantHash {
				t.Errorf("shadowPasswordHash(%q) = %q, want %q", tt.username, hash, tt.wantHash)
			}
		})
	}
}

func TestShadowPasswordHashMissingFile(t *testing.T) {
	origPath := shadowFilePath
	shadowFilePath = "/nonexistent/shadow"
	defer func() { shadowFilePath = origPath }()

	_, err := shadowPasswordHash("root")
	if err == nil {
		t.Error("expected error for missing shadow file")
	}
}

func TestBuildUsermodArgsWithPasswordHash(t *testing.T) {
	// Create a temp shadow file where testuser has an OLD hash.
	tmpFile, err := os.CreateTemp("", "shadow-mod-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "testuser:$6$oldsalt$oldhash:19000:0:99999:7:::\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	tmpFile.Close()

	origPath := shadowFilePath
	shadowFilePath = tmpFile.Name()
	defer func() { shadowFilePath = origPath }()

	existing := &user.User{
		Uid:     "1000",
		Gid:     "1000",
		HomeDir: "/home/testuser",
		Name:    "Test User",
	}

	// Different hash should trigger -p flag.
	newHash := "$6$newsalt$newhash"
	args := buildUsermodArgs("testuser", "", "", "", "", "", newHash, nil, existing)
	if args == nil {
		t.Fatal("expected non-nil args when password hash differs")
	}
	foundP := false
	for i, a := range args {
		if a == "-p" && i+1 < len(args) && args[i+1] == newHash {
			foundP = true
			break
		}
	}
	if !foundP {
		t.Errorf("expected -p %s in args, got %v", newHash, args)
	}

	// Same hash should NOT trigger -p flag.
	sameHash := "$6$oldsalt$oldhash"
	args2 := buildUsermodArgs("testuser", "", "", "", "", "", sameHash, nil, existing)
	if args2 != nil {
		t.Errorf("expected nil args when password hash matches, got %v", args2)
	}
}

func TestUserPresentInvalidPasswordHash(t *testing.T) {
	u := User{
		id:     "test-present-bad-hash",
		method: "present",
		params: map[string]interface{}{
			"name":          "grlx-test-nonexistent-user-abc123",
			"password_hash": "plaintext-not-a-hash",
		},
	}
	result, err := u.present(context.Background(), true)
	if err == nil {
		t.Fatal("expected error for invalid password hash")
	}
	if !result.Failed {
		t.Error("expected failed result for invalid password hash")
	}
}

func TestUserPresentTestModeWithPasswordHash(t *testing.T) {
	u := User{
		id:     "test-present-hash",
		method: "present",
		params: map[string]interface{}{
			"name":          "grlx-test-nonexistent-user-abc123",
			"password_hash": "$6$rounds=5000$salt$hashvalue",
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
		t.Error("expected changed=true in test mode for new user")
	}
	// Verify the command includes -p
	noteStr := result.Notes[0].String()
	if !strings.Contains(noteStr, "-p") {
		t.Errorf("expected -p in useradd command, got %q", noteStr)
	}
}

func TestUserParseWithPasswordHash(t *testing.T) {
	u := User{}
	_, err := u.Parse("test-hash", "present", map[string]interface{}{
		"name":          "testuser",
		"password_hash": "$6$salt$hash",
	})
	if err != nil {
		t.Errorf("Parse() unexpected error: %v", err)
	}
}
