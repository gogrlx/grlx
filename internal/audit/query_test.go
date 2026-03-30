package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestEntries(t *testing.T, logger *Logger, entries []Entry) {
	t.Helper()
	for _, e := range entries {
		if err := logger.Log(e); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
}

func TestQueryEmptyDir(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	result, err := logger.Query(QueryParams{Date: "2026-01-01"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
	if len(result.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(result.Entries))
	}
	if result.Date != "2026-01-01" {
		t.Errorf("date = %q, want 2026-01-01", result.Date)
	}
}

func TestQueryDefaultDate(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	writeTestEntries(t, logger, []Entry{
		{Pubkey: "KEY1", Action: "cook", Success: true},
		{Pubkey: "KEY2", Action: "props.set", Success: true},
	})

	result, err := logger.Query(QueryParams{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if result.Date != today {
		t.Errorf("date = %q, want %q", result.Date, today)
	}
}

func TestQueryFilterAction(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	writeTestEntries(t, logger, []Entry{
		{Pubkey: "KEY1", Action: "cook", Success: true},
		{Pubkey: "KEY1", Action: "props.set", Success: true},
		{Pubkey: "KEY2", Action: "cook", Success: false},
	})

	result, err := logger.Query(QueryParams{Action: "cook"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	for _, e := range result.Entries {
		if e.Action != "cook" {
			t.Errorf("got action %q, want cook", e.Action)
		}
	}
}

func TestQueryFilterPubkey(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	writeTestEntries(t, logger, []Entry{
		{Pubkey: "ALICE", Action: "cook", Success: true},
		{Pubkey: "BOB", Action: "cook", Success: true},
		{Pubkey: "ALICE", Action: "props.set", Success: true},
	})

	result, err := logger.Query(QueryParams{Pubkey: "ALICE"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
}

func TestQueryFailedOnly(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	writeTestEntries(t, logger, []Entry{
		{Pubkey: "KEY1", Action: "cook", Success: true},
		{Pubkey: "KEY1", Action: "pki.accept", Success: false, Error: "denied"},
		{Pubkey: "KEY2", Action: "cook", Success: false, Error: "timeout"},
	})

	result, err := logger.Query(QueryParams{FailedOnly: true})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	for _, e := range result.Entries {
		if e.Success {
			t.Error("got success=true entry in failed_only query")
		}
	}
}

func TestQueryLimit(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	for i := 0; i < 10; i++ {
		writeTestEntries(t, logger, []Entry{
			{Pubkey: "KEY", Action: "cook", Success: true},
		})
	}

	result, err := logger.Query(QueryParams{Limit: 3})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 10 {
		t.Errorf("total = %d, want 10", result.Total)
	}
	if len(result.Entries) != 3 {
		t.Errorf("entries = %d, want 3", len(result.Entries))
	}
}

func TestQueryMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	t1 := time.Date(2026, 3, 13, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 13, 2, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 13, 3, 0, 0, 0, time.UTC)

	writeTestEntries(t, logger, []Entry{
		{Timestamp: t1, Action: "first", Success: true},
		{Timestamp: t2, Action: "second", Success: true},
		{Timestamp: t3, Action: "third", Success: true},
	})

	result, err := logger.Query(QueryParams{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Entries[0].Action != "third" {
		t.Errorf("first entry action = %q, want third", result.Entries[0].Action)
	}
	if result.Entries[2].Action != "first" {
		t.Errorf("last entry action = %q, want first", result.Entries[2].Action)
	}
}

func TestQueryCombinedFilters(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	writeTestEntries(t, logger, []Entry{
		{Pubkey: "ALICE", Action: "cook", Success: true},
		{Pubkey: "ALICE", Action: "cook", Success: false, Error: "fail"},
		{Pubkey: "BOB", Action: "cook", Success: false, Error: "fail"},
		{Pubkey: "ALICE", Action: "props.set", Success: false, Error: "fail"},
	})

	result, err := logger.Query(QueryParams{
		Pubkey:     "ALICE",
		Action:     "cook",
		FailedOnly: true,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

func TestListDatesEmpty(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	dates, err := logger.ListDates()
	if err != nil {
		t.Fatalf("ListDates: %v", err)
	}
	if len(dates) != 0 {
		t.Errorf("dates = %d, want 0", len(dates))
	}
}

func TestListDatesMultiple(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	// Manually create audit files for different dates.
	for _, date := range []string{"2026-03-10", "2026-03-11", "2026-03-12"} {
		entry := Entry{
			Timestamp: time.Now(),
			Pubkey:    "KEY",
			Action:    "cook",
			Success:   true,
		}
		data, _ := json.Marshal(entry)
		data = append(data, '\n')
		path := filepath.Join(dir, "audit-"+date+".jsonl")
		os.WriteFile(path, data, 0o640)
	}

	dates, err := logger.ListDates()
	if err != nil {
		t.Fatalf("ListDates: %v", err)
	}
	if len(dates) != 3 {
		t.Fatalf("dates = %d, want 3", len(dates))
	}
	// Most recent first.
	if dates[0].Date != "2026-03-12" {
		t.Errorf("first date = %q, want 2026-03-12", dates[0].Date)
	}
	if dates[2].Date != "2026-03-10" {
		t.Errorf("last date = %q, want 2026-03-10", dates[2].Date)
	}
	for _, d := range dates {
		if d.EntryCount != 1 {
			t.Errorf("date %s: entry_count = %d, want 1", d.Date, d.EntryCount)
		}
		if d.SizeBytes <= 0 {
			t.Errorf("date %s: size_bytes = %d, want >0", d.Date, d.SizeBytes)
		}
	}
}

func TestQueryMalformedLines(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, "audit-"+today+".jsonl")

	// Write valid and invalid lines.
	content := `{"timestamp":"2026-03-13T01:00:00Z","pubkey":"KEY","action":"cook","success":true}
not json at all
{"timestamp":"2026-03-13T02:00:00Z","pubkey":"KEY","action":"props.set","success":true}
`
	os.WriteFile(path, []byte(content), 0o640)

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	result, err := logger.Query(QueryParams{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2 (malformed line skipped)", result.Total)
	}
}

func TestQueryByUsername(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, fmt.Sprintf("audit-%s.jsonl", today))

	content := `{"timestamp":"2026-03-13T01:00:00Z","pubkey":"KEY1","username":"alice","action":"cook","success":true}
{"timestamp":"2026-03-13T02:00:00Z","pubkey":"KEY2","username":"bob","action":"cook","success":true}
{"timestamp":"2026-03-13T03:00:00Z","pubkey":"KEY1","username":"alice","action":"props.set","success":true}
`
	os.WriteFile(path, []byte(content), 0o640)

	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	result, err := logger.Query(QueryParams{Username: "alice"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2 entries for alice", result.Total)
	}
	for _, e := range result.Entries {
		if e.Username != "alice" {
			t.Errorf("entry username = %q, want alice", e.Username)
		}
	}
}
