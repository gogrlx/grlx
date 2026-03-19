package client

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/audit"
)

func TestListAuditDates_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := []audit.DateSummary{
		{Date: "2026-03-18", EntryCount: 42, SizeBytes: 8192},
		{Date: "2026-03-17", EntryCount: 15, SizeBytes: 3072},
	}
	mockHandler(t, NatsConn, "grlx.api.audit.dates", want)

	got, err := ListAuditDates()
	if err != nil {
		t.Fatalf("ListAuditDates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 dates, got %d", len(got))
	}
	if got[0].Date != "2026-03-18" {
		t.Fatalf("expected 2026-03-18, got %q", got[0].Date)
	}
	if got[0].EntryCount != 42 {
		t.Fatalf("expected 42 entries, got %d", got[0].EntryCount)
	}
}

func TestListAuditDates_Empty(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockHandler(t, NatsConn, "grlx.api.audit.dates", []audit.DateSummary{})

	got, err := ListAuditDates()
	if err != nil {
		t.Fatalf("ListAuditDates: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 dates, got %d", len(got))
	}
}

func TestListAuditDates_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.audit.dates", "audit not configured")

	_, err := ListAuditDates()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryAudit_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := audit.QueryResult{
		Date:  "2026-03-18",
		Total: 2,
		Entries: []audit.Entry{
			{Action: "cook", Pubkey: "NKEY_A"},
			{Action: "pki.accept", Pubkey: "NKEY_B"},
		},
	}
	mockHandler(t, NatsConn, "grlx.api.audit.query", want)

	params := audit.QueryParams{Date: "2026-03-18"}
	got, err := QueryAudit(params)
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	if got.Total != 2 {
		t.Fatalf("expected 2 total, got %d", got.Total)
	}
	if got.Date != "2026-03-18" {
		t.Fatalf("expected date 2026-03-18, got %q", got.Date)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got.Entries))
	}
}

func TestQueryAudit_WithFilters(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := audit.QueryResult{
		Date:    "2026-03-18",
		Total:   1,
		Entries: []audit.Entry{{Action: "cook", Pubkey: "NKEY_A"}},
	}
	mockHandler(t, NatsConn, "grlx.api.audit.query", want)

	params := audit.QueryParams{
		Date:   "2026-03-18",
		Action: "cook",
		Pubkey: "NKEY_A",
	}
	got, err := QueryAudit(params)
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	if got.Total != 1 {
		t.Fatalf("expected 1 total, got %d", got.Total)
	}
}

func TestQueryAudit_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.audit.query", "invalid date format")

	params := audit.QueryParams{Date: "bad-date"}
	_, err := QueryAudit(params)
	if err == nil {
		t.Fatal("expected error")
	}
}
