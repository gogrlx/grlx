package client

import (
	"testing"

	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
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
