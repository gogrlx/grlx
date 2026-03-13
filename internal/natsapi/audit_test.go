package natsapi

import (
	"encoding/json"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/audit"
)

func TestHandleAuditListNoLogger(t *testing.T) {
	// Ensure no global logger is set.
	audit.SetGlobal(nil)

	_, err := handleAuditList(nil)
	if err == nil {
		t.Fatal("expected error when audit logger is not configured")
	}
}

func TestHandleAuditQueryNoLogger(t *testing.T) {
	audit.SetGlobal(nil)

	_, err := handleAuditQuery(nil)
	if err == nil {
		t.Fatal("expected error when audit logger is not configured")
	}
}

func TestHandleAuditListWithLogger(t *testing.T) {
	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	result, err := handleAuditList(nil)
	if err != nil {
		t.Fatalf("handleAuditList: %v", err)
	}

	dates, ok := result.([]audit.DateSummary)
	if !ok {
		t.Fatalf("result type = %T, want []audit.DateSummary", result)
	}
	if len(dates) != 0 {
		t.Errorf("dates = %d, want 0 (empty dir)", len(dates))
	}
}

func TestHandleAuditQueryWithEntries(t *testing.T) {
	dir := t.TempDir()
	logger, err := audit.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()
	audit.SetGlobal(logger)
	defer audit.SetGlobal(nil)

	// Write some entries.
	for _, action := range []string{"cook", "props.set", "cook"} {
		logger.Log(audit.Entry{
			Pubkey:  "TESTKEY",
			Action:  action,
			Success: true,
		})
	}

	// Query all.
	result, err := handleAuditQuery(nil)
	if err != nil {
		t.Fatalf("handleAuditQuery: %v", err)
	}
	qr, ok := result.(audit.QueryResult)
	if !ok {
		t.Fatalf("result type = %T, want audit.QueryResult", result)
	}
	if qr.Total != 3 {
		t.Errorf("total = %d, want 3", qr.Total)
	}

	// Query with filter.
	params, _ := json.Marshal(audit.QueryParams{Action: "cook"})
	result, err = handleAuditQuery(params)
	if err != nil {
		t.Fatalf("handleAuditQuery with filter: %v", err)
	}
	qr = result.(audit.QueryResult)
	if qr.Total != 2 {
		t.Errorf("total = %d, want 2 (cook only)", qr.Total)
	}
}
