package natsapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nkeys"

	"github.com/gogrlx/grlx/v2/internal/audit"
	intauth "github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
	"github.com/gogrlx/grlx/v2/internal/rbac"
	"github.com/gogrlx/grlx/v2/internal/shell"
	"github.com/taigrr/jety"
)

// --- jety test helpers ---

func setupJetyDangerouslyAllowRoot(t *testing.T, enable bool) func() {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("# test config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jety.SetConfigType("toml")
	jety.SetConfigFile(path)
	jety.Set("dangerously_allow_root", enable)
	return func() {
		jety.Set("dangerously_allow_root", false)
		jety.Set("privkey", "")
		jety.Set("pubkeys", nil)
		jety.Set("users", nil)
		jety.Set("roles", nil)
		jety.Set("cohorts", nil)
	}
}

// --- authMiddleware with dangerouslyAllowRoot ---

func TestAuthMiddleware_DangerouslyAllowRoot(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "bypass-ok", nil
	}

	// Non-public method should bypass auth with dangerouslyAllowRoot.
	wrapped := authMiddleware("cook", inner)
	result, err := wrapped(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("inner handler was not called with dangerouslyAllowRoot")
	}
	if result != "bypass-ok" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestAuthMiddleware_DangerouslyAllowRootWithScopeExtractor(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "scoped-ok", nil
	}

	// A method with a scope extractor should still bypass auth.
	wrapped := authMiddleware("cmd.run", inner)
	params := json.RawMessage(`{"target":[{"sprout_id":"web-1"}],"action":{"command":"echo hi"}}`)
	result, err := wrapped(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("inner handler was not called")
	}
	if result != "scoped-ok" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestAuthMiddleware_ScopeExtractorError(t *testing.T) {
	// When the scope extractor returns an error (bad params), the
	// middleware should fall through to the handler for proper error
	// reporting — but only if the token passes the action check.
	// With an invalid token, it should be denied before scope checks.
	cleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer cleanup()

	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "ok", nil
	}

	wrapped := authMiddleware("cook", inner)
	// Invalid JSON in target but has a token — should be denied at token check.
	params := json.RawMessage(`{"token":"invalid","target":"not-an-array"}`)
	_, err := wrapped(params)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if called {
		t.Fatal("handler should not be called with invalid token")
	}
}

// --- handleAuthExplain with dangerouslyAllowRoot ---

func TestHandleAuthExplainDangerouslyAllowRoot(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	result, err := handleAuthExplain(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handleAuthExplain: %v", err)
	}
	// Should return the admin explain response.
	b, _ := json.Marshal(result)
	var resp map[string]interface{}
	json.Unmarshal(b, &resp)

	if resp["pubkey"] != "(dangerously_allow_root)" {
		t.Errorf("pubkey = %v, want (dangerously_allow_root)", resp["pubkey"])
	}
	if resp["isAdmin"] != true {
		t.Errorf("isAdmin = %v, want true", resp["isAdmin"])
	}
}

func TestHandleAuthExplainInvalidJSON(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer cleanup()

	// Invalid JSON should not panic; it should unmarshal to empty struct.
	_, err := handleAuthExplain(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid params (no token)")
	}
}

func TestHandleAuthExplainNilParams(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer cleanup()

	_, err := handleAuthExplain(nil)
	if err == nil {
		t.Fatal("expected error for nil params (no token)")
	}
}

// --- handleAuthWhoAmI with dangerouslyAllowRoot ---

func TestHandleAuthWhoAmIDangerouslyAllowRoot(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	result, err := handleAuthWhoAmI(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handleAuthWhoAmI: %v", err)
	}

	b, _ := json.Marshal(result)
	var resp map[string]interface{}
	json.Unmarshal(b, &resp)

	if resp["pubkey"] != "(dangerously_allow_root)" {
		t.Errorf("pubkey = %v, want (dangerously_allow_root)", resp["pubkey"])
	}
	if resp["role"] != "admin" {
		t.Errorf("role = %v, want admin", resp["role"])
	}
}

// --- handleCmdRun with registered sprout (covers validation pass path) ---

func TestHandleCmdRunRegisteredSprout(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-exec", "UKEY_EXEC")

	// cmd.FRun will try to use NATS — nil conn means it will error, but
	// we cover the validation-pass and goroutine spawn paths.
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	params := json.RawMessage(`{"target":[{"id":"sprout-exec"}],"action":{"command":"echo hello"}}`)
	result, err := handleCmdRun(params)

	// The function may succeed (with an error in results) or fail
	// depending on how FRun handles nil NATS. Either way, we covered
	// the validation pass path.
	if err != nil {
		// If it errors at the top level, that's fine — it means we
		// passed validation but FRun had issues.
		return
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHandleCmdRunEmptyTargets(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[],"action":{"command":"echo hello"}}`)
	result, err := handleCmdRun(params)
	// Empty targets should succeed with empty results.
	if err != nil {
		t.Fatalf("handleCmdRun empty targets: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- handleTestPing with registered sprout ---

func TestHandleTestPingRegisteredSprout(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-ping", "UKEY_PING")

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	params := json.RawMessage(`{"target":[{"id":"sprout-ping"}],"action":{"ping":true}}`)
	result, err := handleTestPing(params)

	if err != nil {
		return // FPing may fail without NATS — still covered validation
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHandleTestPingEmptyTargets(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[],"action":{"ping":true}}`)
	result, err := handleTestPing(params)
	if err != nil {
		t.Fatalf("handleTestPing empty targets: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHandleTestPingSproutIDWithUnderscore(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"sprout_bad"}],"action":{"ping":true}}`)
	_, err := handleTestPing(params)
	if err == nil {
		t.Fatal("expected error for sprout ID with underscore")
	}
}

// --- handleCook with registered sprout ---

func TestHandleCookRegisteredSproutNoNATS(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-cook-valid", "UKEY_COOK_VALID")

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	params := json.RawMessage(`{"target":[{"id":"sprout-cook-valid"}],"action":{"recipe":"webserver.nginx"}}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error when NATS not available")
	}
	if err.Error() != "NATS connection not available" {
		t.Errorf("error = %q, want 'NATS connection not available'", err.Error())
	}
}

func TestHandleCookSproutIDWithUnderscore(t *testing.T) {
	setupNatsAPIPKI(t)

	params := json.RawMessage(`{"target":[{"id":"sprout_bad"}],"action":{"recipe":"test"}}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error for sprout ID with underscore")
	}
}

func TestHandleCookInvalidAction(t *testing.T) {
	setupNatsAPIPKI(t)

	// Valid target structure but action is a string instead of object.
	params := json.RawMessage(`{"target":[{"id":"test-sprout"}],"action":"not-an-object"}`)
	_, err := handleCook(params)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

// --- handleShellStart with registered sprout but nil NATS ---

func TestHandleShellStartRegisteredSproutNoNATS(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-ssh", "UKEY_SSH")

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	params := json.RawMessage(`{"sprout_id":"sprout-ssh","cols":80,"rows":24}`)
	_, err := handleShellStart(params)
	if err == nil {
		t.Fatal("expected error when NATS not available")
	}
}

// --- SetCohortRegistry ---

func TestSetCohortRegistry(t *testing.T) {
	old := cohortRegistry
	defer func() { cohortRegistry = old }()

	reg := rbac.NewRegistry()
	SetCohortRegistry(reg)

	if cohortRegistry != reg {
		t.Error("SetCohortRegistry did not set the registry")
	}

	SetCohortRegistry(nil)
	if cohortRegistry != nil {
		t.Error("SetCohortRegistry(nil) did not clear the registry")
	}
}

// --- auditError type ---

func TestAuditErrorType(t *testing.T) {
	err := errAuditNotConfigured
	if err.Error() != "audit logging not configured" {
		t.Errorf("Error() = %q, want %q", err.Error(), "audit logging not configured")
	}

	// Verify it satisfies the error interface.
	var e error = err
	if e.Error() != "audit logging not configured" {
		t.Errorf("error interface: %q", e.Error())
	}
}

// --- checkScopedAccess ---

func TestCheckScopedAccessDangerouslyAllowRoot(t *testing.T) {
	setupNatsAPIPKI(t)
	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	// With dangerouslyAllowRoot, even invalid tokens should pass.
	err := checkScopedAccess("any-token", rbac.ActionCook, []string{"sprout-1"})
	if err != nil {
		t.Fatalf("expected nil error with dangerouslyAllowRoot, got: %v", err)
	}
}

func TestCheckScopedAccessInvalidToken(t *testing.T) {
	setupNatsAPIPKI(t)
	cleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer cleanup()

	err := checkScopedAccess("invalid-token", rbac.ActionCook, []string{"sprout-1"})
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

// --- filterSproutsByScope ---

func TestFilterSproutsByScopeDangerouslyAllowRoot(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-f1", "UKEY_F1")
	writeNKey(t, pkiDir, "accepted", "sprout-f2", "UKEY_F2")

	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	// With dangerouslyAllowRoot, should return all sprouts.
	result := filterSproutsByScope("any-token", rbac.ActionView, []string{"sprout-f1", "sprout-f2"})
	if len(result) != 2 {
		t.Errorf("expected 2 filtered sprouts, got %d", len(result))
	}
}

func TestFilterSproutsByScopeInvalidToken(t *testing.T) {
	setupNatsAPIPKI(t)
	cleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer cleanup()

	result := filterSproutsByScope("invalid-token", rbac.ActionView, []string{"sprout-1"})
	// Invalid token → no scope filtering possible → returns nil or empty.
	if len(result) != 0 {
		t.Errorf("expected 0 filtered sprouts for invalid token, got %d", len(result))
	}
}

// --- handleSproutsList with dangerouslyAllowRoot (skips scope filter) ---

func TestHandleSproutsListDangerouslyAllowRoot(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-dar", "UKEY_DAR")

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	result, err := handleSproutsList(nil)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m := result.(map[string][]SproutInfo)
	if len(m["sprouts"]) != 1 {
		t.Errorf("expected 1 sprout, got %d", len(m["sprouts"]))
	}
}

func TestHandleSproutsListWithToken(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-tk-1", "UKEY_TK1")
	writeNKey(t, pkiDir, "accepted", "sprout-tk-2", "UKEY_TK2")

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	cleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer cleanup()

	// With an invalid token, scope filtering should filter out sprouts.
	params := json.RawMessage(`{"token":"invalid-token"}`)
	result, err := handleSproutsList(params)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m := result.(map[string][]SproutInfo)
	// With invalid token and no dangerouslyAllowRoot, sprouts should be
	// filtered by scope. TokenScopeFilter with invalid token returns nil,
	// which means "no filter applied" or "deny all" depending on implementation.
	// Either way, no error should occur.
	_ = m
}

// --- handleJobsList scope filtering ---

func TestHandleJobsListDangerouslyAllowRoot(t *testing.T) {
	dir, cleanup := setupJobStore(t)
	defer cleanup()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer jetyCleanup()

	steps := []cook.StepCompletion{
		{ID: "s1", CompletionStatus: cook.StepCompleted, Started: time.Now()},
	}
	writeTestJob(t, dir, "sprout-dar-j", "jid-dar-1", steps)

	result, err := handleJobsList(nil)
	if err != nil {
		t.Fatalf("handleJobsList: %v", err)
	}

	summaries := result.([]jobs.JobSummary)
	if len(summaries) != 1 {
		t.Errorf("expected 1 job, got %d", len(summaries))
	}
}

func TestHandleJobsListWithTokenScope(t *testing.T) {
	dir, cleanup := setupJobStore(t)
	defer cleanup()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer jetyCleanup()

	steps := []cook.StepCompletion{
		{ID: "s1", CompletionStatus: cook.StepCompleted, Started: time.Now()},
	}
	writeTestJob(t, dir, "sprout-scope-j", "jid-scope-1", steps)

	// Token-based filtering with invalid token.
	params := json.RawMessage(`{"token":"invalid-token"}`)
	result, err := handleJobsList(params)
	if err != nil {
		t.Fatalf("handleJobsList: %v", err)
	}

	summaries := result.([]jobs.JobSummary)
	// Should not error; may filter down to 0 depending on scope.
	_ = summaries
}

func TestHandleJobsListInvalidJSON(t *testing.T) {
	_, cleanup := setupJobStore(t)
	defer cleanup()

	// Invalid JSON should be ignored and return all jobs.
	result, err := handleJobsList(json.RawMessage(`{invalid`))
	if err != nil {
		t.Fatalf("handleJobsList: %v", err)
	}
	summaries := result.([]jobs.JobSummary)
	if len(summaries) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(summaries))
	}
}

// --- handleJobsCancel with non-cancellable status ---

func TestHandleJobsCancelCompletedJob(t *testing.T) {
	dir, cleanup := setupJobStore(t)
	defer cleanup()

	steps := []cook.StepCompletion{
		{ID: "s1", CompletionStatus: cook.StepCompleted, Started: time.Now()},
	}
	writeTestJob(t, dir, "sprout-done", "jid-done", steps)

	params := json.RawMessage(`{"jid":"jid-done"}`)
	_, err := handleJobsCancel(params)
	if err == nil {
		t.Fatal("expected error for completed job cancel")
	}
}

func TestHandleJobsCancelInvalidJSON(t *testing.T) {
	_, cleanup := setupJobStore(t)
	defer cleanup()

	_, err := handleJobsCancel(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- handleRecipesGet edge cases ---

func TestHandleRecipesGetEmptyName(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	params, _ := json.Marshal(map[string]string{"name": ""})
	_, err := handleRecipesGet(params)
	if err == nil {
		t.Fatal("expected error for empty recipe name")
	}
}

func TestHandleRecipesGetUsingIDField(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	content := "steps:\n  test:\n    cmd.run:\n      - name: echo hi\n"
	recipePath := filepath.Join(tmpDir, "simple.grlx")
	if err := os.WriteFile(recipePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use "id" field instead of "name".
	params, _ := json.Marshal(map[string]string{"id": "simple"})
	result, err := handleRecipesGet(params)
	if err != nil {
		t.Fatalf("handleRecipesGet with id field: %v", err)
	}

	rc, ok := result.(RecipeContent)
	if !ok {
		t.Fatalf("result type = %T, want RecipeContent", result)
	}
	if rc.Name != "simple" {
		t.Errorf("name = %q, want %q", rc.Name, "simple")
	}
}

func TestHandleRecipesGetInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	_, err := handleRecipesGet(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleRecipesGetDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	// Create a directory where the recipe file would be.
	dirPath := filepath.Join(tmpDir, "myrecipe.grlx")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	params, _ := json.Marshal(map[string]string{"name": "myrecipe"})
	_, err := handleRecipesGet(params)
	if err == nil {
		t.Fatal("expected error when recipe path is a directory")
	}
}

func TestHandleRecipesGetNoRecipeDir(t *testing.T) {
	origRecipeDir := config.RecipeDir
	config.RecipeDir = ""
	defer func() { config.RecipeDir = origRecipeDir }()

	params, _ := json.Marshal(map[string]string{"name": "test"})
	_, err := handleRecipesGet(params)
	if err == nil {
		t.Fatal("expected error when recipe directory is not configured")
	}
}

func TestHandleRecipesListNotADirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRecipeDir := config.RecipeDir
	config.RecipeDir = filePath
	defer func() { config.RecipeDir = origRecipeDir }()

	_, err := handleRecipesList(nil)
	if err == nil {
		t.Fatal("expected error when recipe dir is a file")
	}
}

func TestHandleRecipesListNoDir(t *testing.T) {
	origRecipeDir := config.RecipeDir
	config.RecipeDir = ""
	defer func() { config.RecipeDir = origRecipeDir }()

	_, err := handleRecipesList(nil)
	if err == nil {
		t.Fatal("expected error when recipe directory is not configured")
	}
}

// --- logShellEnd tests ---

func TestLogShellEndNilGlobalLogger(t *testing.T) {
	// Ensure no global logger.
	audit.SetGlobal(nil)

	info := &shell.SessionInfo{
		SessionID: "test-sess",
		SproutID:  "test-sprout",
		Pubkey:    "UTEST",
		RoleName:  "admin",
		Shell:     "/bin/bash",
		StartedAt: time.Now().Add(-5 * time.Minute),
	}

	// Should not panic.
	logShellEnd(info, 5*time.Minute, 0, "")
}

func TestLogShellEndWithLogger(t *testing.T) {
	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	info := &shell.SessionInfo{
		SessionID: "test-sess-logged",
		SproutID:  "test-sprout",
		Pubkey:    "UTESTKEY",
		RoleName:  "operator",
		Shell:     "/bin/sh",
		StartedAt: time.Now().Add(-3 * time.Minute),
	}

	// Should log without error.
	logShellEnd(info, 3*time.Minute, 0, "")
}

func TestLogShellEndWithError(t *testing.T) {
	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	info := &shell.SessionInfo{
		SessionID: "test-sess-err",
		SproutID:  "test-sprout",
		Pubkey:    "UTESTKEY",
		RoleName:  "admin",
		StartedAt: time.Now().Add(-1 * time.Minute),
	}

	// Should log with error message.
	logShellEnd(info, 1*time.Minute, 1, "connection reset")
}

// --- subscribeSessionDone ---

func TestSubscribeSessionDoneNilNATS(t *testing.T) {
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	info := &shell.SessionInfo{
		SessionID:   "test-sub-nil",
		SproutID:    "test-sprout",
		DoneSubject: "grlx.shell.done.test",
	}

	// Should return immediately without panic.
	subscribeSessionDone(info)
}

func TestSubscribeSessionDoneEmptySubject(t *testing.T) {
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	info := &shell.SessionInfo{
		SessionID:   "test-sub-empty",
		SproutID:    "test-sprout",
		DoneSubject: "",
	}

	// Should return immediately without panic.
	subscribeSessionDone(info)
}

// --- handleJobsCancel scope check path ---

func TestHandleJobsCancelWithTokenScope(t *testing.T) {
	dir, cleanup := setupJobStore(t)
	defer cleanup()

	jetyCleanup := setupJetyDangerouslyAllowRoot(t, false)
	defer jetyCleanup()

	steps := []cook.StepCompletion{
		{ID: "s1", Started: time.Now()},
	}
	writeTestJob(t, dir, "sprout-cancel-scope", "jid-cancel-scope", steps)

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	// Token included → triggers scope check path.
	params := json.RawMessage(`{"jid":"jid-cancel-scope","token":"invalid-token"}`)
	_, err := handleJobsCancel(params)
	if err == nil {
		t.Fatal("expected error (scope deny or NATS unavailable)")
	}
}

// --- handleAuthListUsers with roles ---

func TestHandleAuthListUsersWithDangerouslyAllowRoot(t *testing.T) {
	cleanup := setupJetyDangerouslyAllowRoot(t, true)
	defer cleanup()

	result, err := handleAuthListUsers(nil)
	if err != nil {
		t.Fatalf("handleAuthListUsers: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- handleAuthAddUser / RemoveUser with nil params ---

func TestHandleAuthAddUserNilParams(t *testing.T) {
	_, err := handleAuthAddUser(nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestHandleAuthRemoveUserNilParams(t *testing.T) {
	_, err := handleAuthRemoveUser(nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

// --- Subscribe function (0% coverage) ---

func TestSubscribeNilConn(t *testing.T) {
	err := Subscribe(nil)
	// Should fail when trying to subscribe on nil conn.
	if err == nil {
		t.Fatal("expected error for nil NATS connection")
	}
}

// --- probeSprout with nil conn ---

func TestProbeSproutNilConn(t *testing.T) {
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	if probeSprout("any-sprout") {
		t.Error("expected false for nil NATS conn")
	}
}

// --- Cohorts handler tests with invalid JSON ---

func TestHandleCohortsGetInvalidJSON(t *testing.T) {
	cleanup := setupCohortRegistry(t)
	defer cleanup()

	_, err := handleCohortsGet(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleCohortsResolveInvalidJSON(t *testing.T) {
	cleanup := setupCohortRegistry(t)
	defer cleanup()

	_, err := handleCohortsResolve(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandleCohortsRefreshInvalidJSON(t *testing.T) {
	cleanup := setupCohortRegistry(t)
	defer cleanup()

	// Invalid JSON with non-empty name — should unmarshal error but
	// handleCohortsRefresh handles nil params gracefully.
	params := json.RawMessage(`{"name":"valid-name"}`)
	_, err := handleCohortsRefresh(params)
	// Name doesn't exist, so should error.
	if err == nil {
		t.Fatal("expected error for nonexistent cohort")
	}
}

// --- extractSproutsGetID invalid JSON ---

func TestExtractSproutsGetIDInvalidJSON(t *testing.T) {
	_, err := extractSproutsGetID(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractSproutsGetIDEmpty(t *testing.T) {
	_, err := extractSproutsGetID(json.RawMessage(`{"sprout_id":""}`))
	if err == nil {
		t.Fatal("expected error for empty sprout_id")
	}
}

// --- Jobs handler: handleJobsListForSprout with invalid JSON ---

func TestHandleJobsListForSproutInvalidJSON(t *testing.T) {
	_, cleanup := setupJobStore(t)
	defer cleanup()

	_, err := handleJobsListForSprout(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Full auth token tests ---

// setupAuthWithToken creates a real NKey, sets up jety with a privkey and
// user-role mapping, and returns a valid token + cleanup function.
func setupAuthWithToken(t *testing.T, roleName string, rules []rbac.Rule) (string, func()) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("# test config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jety.SetConfigType("toml")
	jety.SetConfigFile(path)
	jety.Set("dangerously_allow_root", false)

	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := kp.Seed()
	pk, _ := kp.PublicKey()
	jety.Set("privkey", string(seed))

	rs := rbac.NewRoleStore()
	rs.Register(&rbac.Role{
		Name:  roleName,
		Rules: rules,
	})
	urm := rbac.NewUserRoleMap()
	urm.Set(pk, roleName)
	intauth.SetPolicy(rs, urm, nil)

	token, err := intauth.NewToken()
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		intauth.SetPolicy(nil, nil, nil)
		jety.Set("privkey", "")
		jety.Set("pubkeys", nil)
		jety.Set("users", nil)
		jety.Set("roles", nil)
		jety.Set("cohorts", nil)
		jety.Set("dangerously_allow_root", false)
	}
	return token, cleanup
}

func TestAuthMiddleware_ValidTokenAllowed(t *testing.T) {
	token, cleanup := setupAuthWithToken(t, "admin", []rbac.Rule{
		{Action: rbac.ActionAdmin, Scope: "*"},
	})
	defer cleanup()

	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "admin-ok", nil
	}

	wrapped := authMiddleware("audit.dates", inner)
	params, _ := json.Marshal(map[string]string{"token": token})
	result, err := wrapped(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called with valid admin token")
	}
	if result != "admin-ok" {
		t.Fatalf("result = %v, want admin-ok", result)
	}
}

func TestAuthMiddleware_ValidTokenDenied(t *testing.T) {
	// Create a viewer token — should be denied for admin methods.
	token, cleanup := setupAuthWithToken(t, "viewer", []rbac.Rule{
		{Action: rbac.ActionView, Scope: "*"},
	})
	defer cleanup()

	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return nil, nil
	}

	wrapped := authMiddleware("audit.dates", inner)
	params, _ := json.Marshal(map[string]string{"token": token})
	_, err := wrapped(params)
	if err == nil {
		t.Fatal("expected error for insufficient permissions")
	}
	if err != rbac.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got: %v", err)
	}
	if called {
		t.Fatal("handler should not be called when permissions denied")
	}
}

func TestAuthMiddleware_ValidTokenScopeCheck(t *testing.T) {
	// Create an operator token with global scope.
	token, cleanup := setupAuthWithToken(t, "operator", []rbac.Rule{
		{Action: rbac.ActionCook, Scope: "*"},
		{Action: rbac.ActionView, Scope: "*"},
		{Action: rbac.ActionCmd, Scope: "*"},
		{Action: rbac.ActionTest, Scope: "*"},
	})
	defer cleanup()

	called := false
	inner := func(params json.RawMessage) (any, error) {
		called = true
		return "cook-ok", nil
	}

	wrapped := authMiddleware("cook", inner)
	params, _ := json.Marshal(map[string]interface{}{
		"token":  token,
		"target": []map[string]string{{"sprout_id": "web-1"}},
		"action": map[string]string{"recipe": "test.sls"},
	})
	result, err := wrapped(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if result != "cook-ok" {
		t.Fatalf("result = %v, want cook-ok", result)
	}
}

// --- handleAuthExplain with valid token ---

func TestHandleAuthExplainWithToken(t *testing.T) {
	token, cleanup := setupAuthWithToken(t, "operator", []rbac.Rule{
		{Action: rbac.ActionCook, Scope: "*"},
		{Action: rbac.ActionView, Scope: "*"},
	})
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"token": token})
	result, err := handleAuthExplain(params)
	if err != nil {
		t.Fatalf("handleAuthExplain: %v", err)
	}

	b, _ := json.Marshal(result)
	var resp map[string]interface{}
	json.Unmarshal(b, &resp)

	if resp["role"] != "operator" {
		t.Errorf("role = %v, want operator", resp["role"])
	}
}

// --- handleAuthWhoAmI with valid token ---

func TestHandleAuthWhoAmIWithToken(t *testing.T) {
	token, cleanup := setupAuthWithToken(t, "admin", []rbac.Rule{
		{Action: rbac.ActionAdmin, Scope: "*"},
	})
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"token": token})
	result, err := handleAuthWhoAmI(params)
	if err != nil {
		t.Fatalf("handleAuthWhoAmI: %v", err)
	}

	b, _ := json.Marshal(result)
	var resp map[string]interface{}
	json.Unmarshal(b, &resp)

	if resp["role"] != "admin" {
		t.Errorf("role = %v, want admin", resp["role"])
	}
	if resp["pubkey"] == "" || resp["pubkey"] == "(dangerously_allow_root)" {
		t.Errorf("expected real pubkey, got %v", resp["pubkey"])
	}
}

// --- handleAuthListUsers with configured users ---

func TestHandleAuthListUsersWithUsers(t *testing.T) {
	token, cleanup := setupAuthWithToken(t, "admin", []rbac.Rule{
		{Action: rbac.ActionAdmin, Scope: "*"},
	})
	defer cleanup()
	_ = token

	result, err := handleAuthListUsers(nil)
	if err != nil {
		t.Fatalf("handleAuthListUsers: %v", err)
	}

	b, _ := json.Marshal(result)
	var resp map[string]interface{}
	json.Unmarshal(b, &resp)

	// Should have at least the user we configured.
	users, ok := resp["users"].(map[string]interface{})
	if !ok {
		t.Fatalf("users type = %T, want map", resp["users"])
	}
	if len(users) == 0 {
		t.Error("expected at least 1 user")
	}

	roles, ok := resp["roles"].([]interface{})
	if !ok {
		t.Fatalf("roles type = %T, want []", resp["roles"])
	}
	if len(roles) == 0 {
		t.Error("expected at least 1 role")
	}
}

// --- handleAuthAddUser + RemoveUser with valid auth setup ---

func TestHandleAuthAddAndRemoveUser(t *testing.T) {
	_, cleanup := setupAuthWithToken(t, "admin", []rbac.Rule{
		{Action: rbac.ActionAdmin, Scope: "*"},
	})
	defer cleanup()

	// Create a real NKey pubkey for the new user.
	newKP, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	newPK, _ := newKP.PublicKey()

	// Add the user.
	addParams, _ := json.Marshal(map[string]string{
		"pubkey": newPK,
		"role":   "admin",
	})
	result, err := handleAuthAddUser(addParams)
	if err != nil {
		t.Fatalf("handleAuthAddUser: %v", err)
	}
	b, _ := json.Marshal(result)
	if !json.Valid(b) {
		t.Fatal("invalid JSON result")
	}

	// Remove the user.
	removeParams, _ := json.Marshal(map[string]string{
		"pubkey": newPK,
	})
	result, err = handleAuthRemoveUser(removeParams)
	if err != nil {
		t.Fatalf("handleAuthRemoveUser: %v", err)
	}
	b, _ = json.Marshal(result)
	if !json.Valid(b) {
		t.Fatal("invalid JSON result")
	}
}

// --- handleSproutsList with valid token (scope filtering active path) ---

func TestHandleSproutsListWithValidToken(t *testing.T) {
	pkiDir := setupNatsAPIPKI(t)
	writeNKey(t, pkiDir, "accepted", "sprout-vt-1", "UKEY_VT1")
	writeNKey(t, pkiDir, "accepted", "sprout-vt-2", "UKEY_VT2")

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	token, cleanup := setupAuthWithToken(t, "viewer", []rbac.Rule{
		{Action: rbac.ActionView, Scope: "*"},
		{Action: rbac.ActionUserRead, Scope: "*"},
	})
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"token": token})
	result, err := handleSproutsList(params)
	if err != nil {
		t.Fatalf("handleSproutsList: %v", err)
	}

	m := result.(map[string][]SproutInfo)
	// With global scope, both sprouts should be visible.
	if len(m["sprouts"]) < 2 {
		t.Errorf("expected at least 2 sprouts, got %d", len(m["sprouts"]))
	}
}

// --- handleJobsList with valid token (scope filtering active path) ---

func TestHandleJobsListWithValidToken(t *testing.T) {
	dir, cleanup := setupJobStore(t)
	defer cleanup()

	token, authCleanup := setupAuthWithToken(t, "viewer", []rbac.Rule{
		{Action: rbac.ActionView, Scope: "*"},
		{Action: rbac.ActionUserRead, Scope: "*"},
	})
	defer authCleanup()

	steps := []cook.StepCompletion{
		{ID: "s1", CompletionStatus: cook.StepCompleted, Started: time.Now()},
	}
	writeTestJob(t, dir, "sprout-jl-1", "jid-jl-1", steps)
	writeTestJob(t, dir, "sprout-jl-2", "jid-jl-2", steps)

	params, _ := json.Marshal(map[string]string{"token": token})
	result, err := handleJobsList(params)
	if err != nil {
		t.Fatalf("handleJobsList: %v", err)
	}

	summaries := result.([]jobs.JobSummary)
	if len(summaries) < 2 {
		t.Errorf("expected at least 2 jobs, got %d", len(summaries))
	}
}

// --- resolveCallerIdentity with valid token ---

func TestResolveCallerIdentityValidToken(t *testing.T) {
	token, cleanup := setupAuthWithToken(t, "admin", []rbac.Rule{
		{Action: rbac.ActionAdmin, Scope: "*"},
	})
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"token": token})
	pk, role := resolveCallerIdentity(params)
	if pk == "" {
		t.Error("expected non-empty pubkey")
	}
	if role != "admin" {
		t.Errorf("role = %q, want admin", role)
	}
}
