package client

import (
	"testing"

	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/audit"
	"github.com/gogrlx/grlx/v2/internal/jobs"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// mockPKIHandlers sets up both pki.list and a target pki action handler.
// listResp is the response for pki.list; actionSubject is the pki action
// (e.g. "grlx.api.pki.accept"). The action handler replies with success.
func mockPKIHandlers(t *testing.T, nc *nats.Conn, listResp pki.KeysByType, actionSubject string) {
	t.Helper()
	mockHandler(t, nc, "grlx.api.pki.list", listResp)
	mockHandler(t, nc, actionSubject, map[string]bool{"ok": true})
}

func keysWithSprout(state string, ids ...string) pki.KeysByType {
	sprouts := make([]pki.KeyManager, len(ids))
	for i, id := range ids {
		sprouts[i] = pki.KeyManager{SproutID: id}
	}
	ks := pki.KeysByType{}
	set := pki.KeySet{Sprouts: sprouts}
	switch state {
	case "accepted":
		ks.Accepted = set
	case "unaccepted":
		ks.Unaccepted = set
	case "denied":
		ks.Denied = set
	case "rejected":
		ks.Rejected = set
	}
	// Fill other sets with empty sprout slices to avoid nil
	if ks.Accepted.Sprouts == nil {
		ks.Accepted = pki.KeySet{Sprouts: []pki.KeyManager{}}
	}
	if ks.Unaccepted.Sprouts == nil {
		ks.Unaccepted = pki.KeySet{Sprouts: []pki.KeyManager{}}
	}
	if ks.Denied.Sprouts == nil {
		ks.Denied = pki.KeySet{Sprouts: []pki.KeyManager{}}
	}
	if ks.Rejected.Sprouts == nil {
		ks.Rejected = pki.KeySet{Sprouts: []pki.KeyManager{}}
	}
	return ks
}

func TestListKeys_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := keysWithSprout("accepted", "web-01", "web-02")
	want.Unaccepted = pki.KeySet{Sprouts: []pki.KeyManager{{SproutID: "new-sprout"}}}
	mockHandler(t, NatsConn, "grlx.api.pki.list", want)

	got, err := ListKeys()
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(got.Accepted.Sprouts) != 2 {
		t.Fatalf("expected 2 accepted, got %d", len(got.Accepted.Sprouts))
	}
	if len(got.Unaccepted.Sprouts) != 1 {
		t.Fatalf("expected 1 unaccepted, got %d", len(got.Unaccepted.Sprouts))
	}
}

func TestListKeys_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.pki.list", "NATS error")

	_, err := ListKeys()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAcceptKey_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("unaccepted", "new-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.accept")

	ok, err := AcceptKey("new-sprout")
	if err != nil {
		t.Fatalf("AcceptKey: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestAcceptKey_AlreadyAccepted(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := AcceptKey("web-01")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrAlreadyAccepted {
		t.Fatalf("expected ErrAlreadyAccepted, got: %v", err)
	}
}

func TestAcceptKey_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := AcceptKey("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrSproutIDNotFound {
		t.Fatalf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestAcceptKey_FromDenied(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("denied", "bad-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.accept")

	ok, err := AcceptKey("bad-sprout")
	if err != nil {
		t.Fatalf("AcceptKey from denied: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestUnacceptKey_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.unaccept")

	ok, err := UnacceptKey("web-01")
	if err != nil {
		t.Fatalf("UnacceptKey: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestUnacceptKey_AlreadyUnaccepted(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("unaccepted", "new-sprout")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := UnacceptKey("new-sprout")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrAlreadyUnaccepted {
		t.Fatalf("expected ErrAlreadyUnaccepted, got: %v", err)
	}
}

func TestRejectKey_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "compromised")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.reject")

	ok, err := RejectKey("compromised")
	if err != nil {
		t.Fatalf("RejectKey: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestRejectKey_AlreadyRejected(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("rejected", "old-sprout")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := RejectKey("old-sprout")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrAlreadyRejected {
		t.Fatalf("expected ErrAlreadyRejected, got: %v", err)
	}
}

func TestDenyKey_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("unaccepted", "suspicious")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.deny")

	ok, err := DenyKey("suspicious")
	if err != nil {
		t.Fatalf("DenyKey: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDenyKey_AlreadyDenied(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("denied", "bad-actor")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := DenyKey("bad-actor")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrAlreadyDenied {
		t.Fatalf("expected ErrAlreadyDenied, got: %v", err)
	}
}

func TestDeleteKey_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("rejected", "decommissioned")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.delete")

	ok, err := DeleteKey("decommissioned")
	if err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDeleteKey_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := DeleteKey("ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrSproutIDNotFound {
		t.Fatalf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

// TestResolveTargets_Regex tests the full ResolveTargets path with regex matching.
func TestResolveTargets_Regex(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01", "web-02", "db-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	got, err := ResolveTargets("web-.*")
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
}

// TestResolveTargets_CommaList tests target resolution with comma-separated list.
func TestResolveTargets_CommaList(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01", "web-02", "db-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	got, err := ResolveTargets("web-01,db-01")
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
}

// TestResolveTargets_NoMatch tests when regex matches nothing.
func TestResolveTargets_NoMatch(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	got, err := ResolveTargets("db-.*")
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 targets, got %d", len(got))
	}
}

// TestCook_ParamsMarshal verifies the cook request is properly structured.
func TestCook_NATSError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockErrorHandler(t, NatsConn, "grlx.api.cook", "recipe not found")

	// Validate that cook propagates API errors
	_, err := Cook("web-01", dummyCmdCook())
	if err == nil {
		t.Fatal("expected error")
	}
}

func dummyCmdCook() apitypes.CmdCook {
	return apitypes.CmdCook{
		Env:    "base",
		Recipe: "test-recipe",
		Test:   true,
	}
}

func TestCook_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01", "web-02")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	want := apitypes.CmdCook{
		Env:    "base",
		Recipe: "test-recipe",
		Test:   true,
	}
	mockHandler(t, NatsConn, "grlx.api.cook", want)

	got, err := Cook("web-.*", dummyCmdCook())
	if err != nil {
		t.Fatalf("Cook: %v", err)
	}
	if got.Recipe != "test-recipe" {
		t.Fatalf("expected recipe test-recipe, got %q", got.Recipe)
	}
}

func TestCook_ResolveError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	// ListKeys fails → ResolveTargets fails → Cook fails.
	mockErrorHandler(t, NatsConn, "grlx.api.pki.list", "NATS down")

	_, err := Cook("web-.*", dummyCmdCook())
	if err == nil {
		t.Fatal("expected error from ResolveTargets")
	}
}

func TestCook_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockBadJSONHandler(t, NatsConn, "grlx.api.cook")

	_, err := Cook("web-01", dummyCmdCook())
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- Unmarshal error paths for PKI, sprouts, audit, cohorts, version ---

func TestListKeys_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.pki.list")

	_, err := ListKeys()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListSprouts_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.sprouts.list")

	_, err := ListSprouts()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetSprout_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.sprouts.get")

	_, err := GetSprout("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetSproutProps_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.props.getall")

	_, err := GetSproutProps("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListAuditDates_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.audit.dates")

	_, err := ListAuditDates()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestQueryAudit_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.audit.query")

	_, err := QueryAudit(audit.QueryParams{Date: "2026-03-18"})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetCohort_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.get")

	_, err := GetCohort("web-servers")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestResolveCohort_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.resolve")

	_, err := ResolveCohort("web-servers")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRefreshCohort_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.refresh")

	_, err := RefreshCohort("web-servers")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestRefreshAllCohorts_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.cohorts.refresh", "internal error")

	_, err := RefreshAllCohorts()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshAllCohorts_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.cohorts.refresh")

	_, err := RefreshAllCohorts()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetVersion_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.version")

	_, err := GetVersion()
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- Additional PKI error paths ---

func TestAcceptKey_ListKeysError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.pki.list", "NATS timeout")

	_, err := AcceptKey("web-01")
	if err == nil {
		t.Fatal("expected error from ListKeys")
	}
}

func TestUnacceptKey_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := UnacceptKey("ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrSproutIDNotFound {
		t.Fatalf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestRejectKey_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := RejectKey("ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrSproutIDNotFound {
		t.Fatalf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

func TestDenyKey_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := DenyKey("ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if err != pki.ErrSproutIDNotFound {
		t.Fatalf("expected ErrSproutIDNotFound, got: %v", err)
	}
}

// --- ResolveTargets edge cases ---

func TestResolveTargets_InvalidRegex(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	_, err := ResolveTargets("[invalid")
	if err == nil {
		t.Fatal("expected regex compilation error")
	}
}

func TestResolveTargets_ExactMatch(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01", "web-010")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	got, err := ResolveTargets("web-01")
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 target (exact match), got %d: %v", len(got), got)
	}
	if got[0] != "web-01" {
		t.Fatalf("expected web-01, got %q", got[0])
	}
}

func TestResolveTargets_CommaSingle(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01", "web-02")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)

	// Trailing comma forces list mode, not regex.
	got, err := ResolveTargets("web-01,")
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 target, got %d: %v", len(got), got)
	}
}

func TestResolveTargets_ListKeysError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.pki.list", "auth failed")

	_, err := ResolveTargets("web-.*")
	if err == nil {
		t.Fatal("expected error from ListKeys")
	}
}

// --- Jobs unmarshal error paths ---

func TestListJobs_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.jobs.list")

	_, err := ListJobs(10, "")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGetJob_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.jobs.get")

	_, err := GetJob("jid-001")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListJobsForSprout_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.jobs.forsprout")

	_, err := ListJobsForSprout("web-01")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestListJobsForSprout_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.jobs.forsprout", "sprout not found")

	_, err := ListJobsForSprout("ghost")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- PKI NATS action error paths ---

func TestAcceptKey_NATSActionError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("unaccepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockErrorHandler(t, NatsConn, "grlx.api.pki.accept", "storage failure")

	_, err := AcceptKey("web-01")
	if err == nil {
		t.Fatal("expected error from pki.accept action")
	}
}

func TestUnacceptKey_NATSActionError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockErrorHandler(t, NatsConn, "grlx.api.pki.unaccept", "storage failure")

	_, err := UnacceptKey("web-01")
	if err == nil {
		t.Fatal("expected error from pki.unaccept action")
	}
}

func TestRejectKey_NATSActionError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockErrorHandler(t, NatsConn, "grlx.api.pki.reject", "storage failure")

	_, err := RejectKey("web-01")
	if err == nil {
		t.Fatal("expected error from pki.reject action")
	}
}

func TestDenyKey_NATSActionError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockErrorHandler(t, NatsConn, "grlx.api.pki.deny", "storage failure")

	_, err := DenyKey("web-01")
	if err == nil {
		t.Fatal("expected error from pki.deny action")
	}
}

func TestDeleteKey_NATSActionError(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "web-01")
	mockHandler(t, NatsConn, "grlx.api.pki.list", keys)
	mockErrorHandler(t, NatsConn, "grlx.api.pki.delete", "storage failure")

	_, err := DeleteKey("web-01")
	if err == nil {
		t.Fatal("expected error from pki.delete action")
	}
}

// --- Additional key state transitions ---

func TestAcceptKey_FromRejected(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("rejected", "old-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.accept")

	ok, err := AcceptKey("old-sprout")
	if err != nil {
		t.Fatalf("AcceptKey from rejected: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestUnacceptKey_FromDenied(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("denied", "bad-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.unaccept")

	ok, err := UnacceptKey("bad-sprout")
	if err != nil {
		t.Fatalf("UnacceptKey from denied: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestUnacceptKey_FromRejected(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("rejected", "old-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.unaccept")

	ok, err := UnacceptKey("old-sprout")
	if err != nil {
		t.Fatalf("UnacceptKey from rejected: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestRejectKey_FromUnaccepted(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("unaccepted", "new-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.reject")

	ok, err := RejectKey("new-sprout")
	if err != nil {
		t.Fatalf("RejectKey from unaccepted: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestRejectKey_FromDenied(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("denied", "bad-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.reject")

	ok, err := RejectKey("bad-sprout")
	if err != nil {
		t.Fatalf("RejectKey from denied: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDenyKey_FromAccepted(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "compromised")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.deny")

	ok, err := DenyKey("compromised")
	if err != nil {
		t.Fatalf("DenyKey from accepted: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDenyKey_FromRejected(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("rejected", "old-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.deny")

	ok, err := DenyKey("old-sprout")
	if err != nil {
		t.Fatalf("DenyKey from rejected: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDeleteKey_FromAccepted(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("accepted", "decommissioned")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.delete")

	ok, err := DeleteKey("decommissioned")
	if err != nil {
		t.Fatalf("DeleteKey from accepted: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDeleteKey_FromUnaccepted(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("unaccepted", "new-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.delete")

	ok, err := DeleteKey("new-sprout")
	if err != nil {
		t.Fatalf("DeleteKey from unaccepted: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestDeleteKey_FromDenied(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	keys := keysWithSprout("denied", "bad-sprout")
	mockPKIHandlers(t, NatsConn, keys, "grlx.api.pki.delete")

	ok, err := DeleteKey("bad-sprout")
	if err != nil {
		t.Fatalf("DeleteKey from denied: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

// --- ListJobs with user filter ---

func TestListJobs_WithUser(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := []jobs.JobSummary{
		{JID: "jid-user", SproutID: "web-01", Status: jobs.JobSucceeded, Total: 1, Succeeded: 1},
	}
	mockHandler(t, NatsConn, "grlx.api.jobs.list", want)

	got, err := ListJobs(10, "NKEY_ALICE")
	if err != nil {
		t.Fatalf("ListJobs with user: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 job, got %d", len(got))
	}
}
