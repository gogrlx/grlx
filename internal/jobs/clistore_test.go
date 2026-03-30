package jobs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func TestCLIStore_RecordJobStart(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatalf("NewCLIStore: %v", err)
	}

	meta := CLIJobMeta{
		JID:       "job-001",
		SproutID:  "sprout-a",
		Recipe:    "web.deploy",
		UserKey:   "UABC123",
		CreatedAt: time.Now(),
	}

	if err := store.RecordJobStart(meta); err != nil {
		t.Fatalf("RecordJobStart: %v", err)
	}

	// Verify meta file exists.
	metaPath := filepath.Join(dir, "sprout-a", "job-001.meta.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("meta file should exist: %v", err)
	}

	// Verify JSONL file exists.
	jsonlPath := filepath.Join(dir, "sprout-a", "job-001.jsonl")
	if _, err := os.Stat(jsonlPath); err != nil {
		t.Fatalf("jsonl file should exist: %v", err)
	}
}

func TestCLIStore_AppendStep(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatalf("NewCLIStore: %v", err)
	}

	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepCompleted,
		ChangesMade:      true,
		Changes:          []string{"installed package"},
		Started:          time.Now(),
		Duration:         2 * time.Second,
	}

	if err := store.AppendStep("sprout-a", "job-002", step); err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	// Append another step.
	step2 := cook.StepCompletion{
		ID:               "step-2",
		CompletionStatus: cook.StepFailed,
		Started:          time.Now(),
		Duration:         500 * time.Millisecond,
	}
	if err := store.AppendStep("sprout-a", "job-002", step2); err != nil {
		t.Fatalf("AppendStep second: %v", err)
	}

	// Read and verify.
	jobFile := filepath.Join(dir, "sprout-a", "job-002.jsonl")
	data, err := os.ReadFile(jobFile)
	if err != nil {
		t.Fatalf("reading job file: %v", err)
	}
	lines := 0
	for _, line := range splitNonEmpty(string(data)) {
		if line != "" {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("expected 2 lines, got %d", lines)
	}
}

func TestCLIStore_GetJob(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatalf("NewCLIStore: %v", err)
	}

	meta := CLIJobMeta{
		JID:       "job-003",
		SproutID:  "sprout-b",
		Recipe:    "base.setup",
		UserKey:   "UXYZ789",
		CreatedAt: time.Now(),
	}
	if err := store.RecordJobStart(meta); err != nil {
		t.Fatal(err)
	}

	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	if err := store.AppendStep("sprout-b", "job-003", step); err != nil {
		t.Fatal(err)
	}

	summary, gotMeta, err := store.GetJob("job-003")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary.JID != "job-003" {
		t.Errorf("expected JID job-003, got %s", summary.JID)
	}
	if summary.SproutID != "sprout-b" {
		t.Errorf("expected sprout-b, got %s", summary.SproutID)
	}
	if summary.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", summary.Succeeded)
	}
	if gotMeta == nil {
		t.Fatal("expected meta, got nil")
	}
	if gotMeta.UserKey != "UXYZ789" {
		t.Errorf("expected user key UXYZ789, got %s", gotMeta.UserKey)
	}
	if gotMeta.Recipe != "base.setup" {
		t.Errorf("expected recipe base.setup, got %s", gotMeta.Recipe)
	}
}

func TestCLIStore_GetJob_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.GetJob("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestCLIStore_ListJobs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create jobs for two different users.
	for _, tc := range []struct {
		jid     string
		sprout  string
		userKey string
	}{
		{"job-a", "sprout-1", "USER_A"},
		{"job-b", "sprout-1", "USER_B"},
		{"job-c", "sprout-2", "USER_A"},
	} {
		meta := CLIJobMeta{
			JID:       tc.jid,
			SproutID:  tc.sprout,
			UserKey:   tc.userKey,
			CreatedAt: time.Now(),
		}
		if err := store.RecordJobStart(meta); err != nil {
			t.Fatal(err)
		}
		step := cook.StepCompletion{
			ID:               "step-1",
			CompletionStatus: cook.StepCompleted,
			Started:          time.Now(),
			Duration:         time.Second,
		}
		if err := store.AppendStep(tc.sprout, tc.jid, step); err != nil {
			t.Fatal(err)
		}
	}

	// List all.
	all, err := store.ListJobs(0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(all))
	}

	// Filter by user.
	userA, err := store.ListJobs(0, "USER_A", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(userA) != 2 {
		t.Errorf("expected 2 jobs for USER_A, got %d", len(userA))
	}

	userB, err := store.ListJobs(0, "USER_B", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(userB) != 1 {
		t.Errorf("expected 1 job for USER_B, got %d", len(userB))
	}

	// Test limit.
	limited, err := store.ListJobs(1, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1 job with limit, got %d", len(limited))
	}

	// Filter by sprout.
	sprout1, err := store.ListJobs(0, "", "sprout-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sprout1) != 2 {
		t.Errorf("expected 2 jobs for sprout-1, got %d", len(sprout1))
	}

	sprout2, err := store.ListJobs(0, "", "sprout-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(sprout2) != 1 {
		t.Errorf("expected 1 job for sprout-2, got %d", len(sprout2))
	}

	// Filter by both user and sprout.
	userASprout1, err := store.ListJobs(0, "USER_A", "sprout-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(userASprout1) != 1 {
		t.Errorf("expected 1 job for USER_A on sprout-1, got %d", len(userASprout1))
	}

	// Non-existent sprout returns empty.
	none, err := store.ListJobs(0, "", "sprout-nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 jobs for nonexistent sprout, got %d", len(none))
	}
}

func TestCLIStore_GetJobMeta(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	meta := CLIJobMeta{
		JID:       "job-meta-test",
		SproutID:  "sprout-x",
		Recipe:    "test.recipe",
		UserKey:   "UKEY123",
		CreatedAt: time.Now(),
	}
	if err := store.RecordJobStart(meta); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetJobMeta("job-meta-test")
	if err != nil {
		t.Fatalf("GetJobMeta: %v", err)
	}
	if got.UserKey != "UKEY123" {
		t.Errorf("expected UKEY123, got %s", got.UserKey)
	}
}

func TestCLIStore_GetJobMeta_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.GetJobMeta("nope")
	if err != ErrMetaNotFound {
		t.Errorf("expected ErrMetaNotFound, got %v", err)
	}
}

func TestCLIStore_DeleteJob(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	meta := CLIJobMeta{
		JID:      "del-job",
		SproutID: "sprout-del",
		UserKey:  "UKEY",
	}
	if err := store.RecordJobStart(meta); err != nil {
		t.Fatal(err)
	}
	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	if err := store.AppendStep("sprout-del", "del-job", step); err != nil {
		t.Fatal(err)
	}

	// Verify it exists.
	_, _, err = store.GetJob("del-job")
	if err != nil {
		t.Fatalf("job should exist before delete: %v", err)
	}

	// Delete it.
	if err := store.DeleteJob("del-job"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Should be gone.
	_, _, err = store.GetJob("del-job")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound after delete, got %v", err)
	}

	// Empty sprout dir should be cleaned up.
	sproutDir := filepath.Join(dir, "sprout-del")
	if _, statErr := os.Stat(sproutDir); !os.IsNotExist(statErr) {
		t.Error("expected empty sprout dir to be removed")
	}
}

func TestCLIStore_DeleteJob_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = store.DeleteJob("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestCLIStore_Stats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Empty store.
	stats, err := store.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalJobs != 0 || stats.TotalSprouts != 0 || stats.DiskBytes != 0 {
		t.Errorf("expected zero stats, got %+v", stats)
	}

	// Add some jobs.
	for _, tc := range []struct {
		jid    string
		sprout string
	}{
		{"j1", "s1"},
		{"j2", "s1"},
		{"j3", "s2"},
	} {
		meta := CLIJobMeta{JID: tc.jid, SproutID: tc.sprout, UserKey: "U"}
		if err := store.RecordJobStart(meta); err != nil {
			t.Fatal(err)
		}
		step := cook.StepCompletion{
			ID: "step-1", CompletionStatus: cook.StepCompleted,
			Started: time.Now(), Duration: time.Second,
		}
		if err := store.AppendStep(tc.sprout, tc.jid, step); err != nil {
			t.Fatal(err)
		}
	}

	stats, err = store.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalJobs != 3 {
		t.Errorf("expected 3 jobs, got %d", stats.TotalJobs)
	}
	if stats.TotalSprouts != 2 {
		t.Errorf("expected 2 sprouts, got %d", stats.TotalSprouts)
	}
	if stats.DiskBytes <= 0 {
		t.Errorf("expected positive disk usage, got %d", stats.DiskBytes)
	}
}

func TestCLIStore_Purge(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create two jobs.
	for _, jid := range []string{"old-job", "new-job"} {
		meta := CLIJobMeta{JID: jid, SproutID: "sprout-p", UserKey: "U"}
		if err := store.RecordJobStart(meta); err != nil {
			t.Fatal(err)
		}
		step := cook.StepCompletion{
			ID: "step-1", CompletionStatus: cook.StepCompleted,
			Started: time.Now(), Duration: time.Second,
		}
		if err := store.AppendStep("sprout-p", jid, step); err != nil {
			t.Fatal(err)
		}
	}

	// Backdate old-job files.
	past := time.Now().Add(-48 * time.Hour)
	sproutDir := filepath.Join(dir, "sprout-p")
	os.Chtimes(filepath.Join(sproutDir, "old-job.jsonl"), past, past)
	os.Chtimes(filepath.Join(sproutDir, "old-job.meta.json"), past, past)

	// Purge jobs older than 24h.
	removed, err := store.Purge(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// old-job should be gone, new-job should remain.
	_, _, err = store.GetJob("old-job")
	if err != ErrJobNotFound {
		t.Errorf("expected old-job to be purged, got %v", err)
	}
	_, _, err = store.GetJob("new-job")
	if err != nil {
		t.Errorf("new-job should still exist: %v", err)
	}
}

func TestCLIStore_StartReaper_ZeroTTL(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a job and backdate it.
	meta := CLIJobMeta{JID: "reaper-test", SproutID: "sr", UserKey: "U"}
	if err := store.RecordJobStart(meta); err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-9999 * time.Hour)
	os.Chtimes(filepath.Join(dir, "sr", "reaper-test.jsonl"), past, past)

	// Zero TTL should not start reaper — file should survive.
	store.StartReaper(0)
	time.Sleep(50 * time.Millisecond)

	if _, err := os.Stat(filepath.Join(dir, "sr", "reaper-test.jsonl")); err != nil {
		t.Errorf("job file should still exist with TTL=0: %v", err)
	}
}

// splitNonEmpty splits a string by newline and filters empty lines.
func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range splitLines(s) {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
