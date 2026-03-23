package jobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func TestNewStore_Default(t *testing.T) {
	// NewStore uses config.JobLogDir; verify it returns a non-nil store.
	store := NewStore()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestJobStatus_String_Unknown(t *testing.T) {
	var s JobStatus = 99
	if s.String() != "unknown" {
		t.Errorf("expected 'unknown', got %q", s.String())
	}
}

func TestJobStatus_UnmarshalJSON_Invalid(t *testing.T) {
	var s JobStatus
	err := s.UnmarshalJSON([]byte(`"bogus"`))
	if err == nil {
		t.Error("expected error for unknown status string")
	}
}

func TestJobStatus_UnmarshalJSON_NotString(t *testing.T) {
	var s JobStatus
	err := s.UnmarshalJSON([]byte(`123`))
	if err == nil {
		t.Error("expected error for non-string JSON")
	}
}

func TestReadJobFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(jobFile, []byte("not json at all\n"), 0o644)

	_, err := readJobFile(jobFile)
	if err == nil {
		t.Error("expected error for invalid JSON in job file")
	}
}

func TestListSproutDirs_EmptyLogDir(t *testing.T) {
	store := NewStoreWithDir("")
	_, err := store.listSproutDirs()
	if err != ErrInvalidJobDir {
		t.Errorf("expected ErrInvalidJobDir, got %v", err)
	}
}

func TestGetJob_ReadError(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	// Create a sprout dir with a directory named like a job file.
	sproutDir := filepath.Join(dir, "sprout-err")
	os.MkdirAll(filepath.Join(sproutDir, "job-dir.jsonl"), 0o700)

	_, err := store.GetJob("sprout-err", "job-dir")
	if err == nil {
		t.Error("expected error when job file is a directory")
	}
}

func TestFindJob_EmptyLogDir(t *testing.T) {
	store := NewStoreWithDir("")
	_, err := store.FindJob("any-jid")
	if err != ErrInvalidJobDir {
		t.Errorf("expected ErrInvalidJobDir, got %v", err)
	}
}

func TestListAllJobs_InvalidLogDir(t *testing.T) {
	store := NewStoreWithDir("")
	_, err := store.ListAllJobs(0)
	if err != ErrInvalidJobDir {
		t.Errorf("expected ErrInvalidJobDir, got %v", err)
	}
}

func TestListSprouts_InvalidLogDir(t *testing.T) {
	store := NewStoreWithDir("")
	_, err := store.ListSprouts()
	if err != ErrInvalidJobDir {
		t.Errorf("expected ErrInvalidJobDir, got %v", err)
	}
}

func TestCountJobsForSprout_IgnoresNonJsonl(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-mixed")
	os.MkdirAll(sproutDir, 0o700)

	// Create a .jsonl file, a .meta.json file, and a subdirectory.
	os.WriteFile(filepath.Join(sproutDir, "job.jsonl"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(sproutDir, "job.meta.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(sproutDir, "subdir"), 0o700)

	count, err := store.CountJobsForSprout("sprout-mixed")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 (only .jsonl), got %d", count)
	}
}

func TestListJobsForSprout_SkipsDirs(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	sproutDir := filepath.Join(dir, "sprout-dirs")
	os.MkdirAll(sproutDir, 0o700)

	// Create a real job.
	writeJobFile(t, dir, "sprout-dirs", "real-job", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})

	// Create a subdirectory that should be ignored.
	os.MkdirAll(filepath.Join(sproutDir, "not-a-job"), 0o700)
	// Create a non-jsonl file.
	os.WriteFile(filepath.Join(sproutDir, "readme.txt"), []byte("hi"), 0o644)

	summaries, err := store.ListJobsForSprout("sprout-dirs")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 job, got %d", len(summaries))
	}
}

func TestBuildSummary_ZeroStartTimes(t *testing.T) {
	steps := []cook.StepCompletion{
		makeStep("s1", cook.StepNotStarted, time.Time{}, 0),
		makeStep("s2", cook.StepNotStarted, time.Time{}, 0),
	}

	summary := buildSummary("jid", "sprout", steps)
	if !summary.StartedAt.IsZero() {
		t.Error("expected zero StartedAt for not-started steps")
	}
	if summary.Duration != 0 {
		t.Errorf("expected zero duration, got %v", summary.Duration)
	}
}

func TestBuildSummary_Skipped(t *testing.T) {
	now := time.Now()
	steps := []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
		makeStep("s2", cook.StepSkipped, now, 0),
	}

	summary := buildSummary("jid", "sprout", steps)
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", summary.Skipped)
	}
	if summary.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", summary.Succeeded)
	}
}

func TestListAllJobs_WithInvokedBy(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	writeJobFile(t, dir, "sprout-allinv", "job-inv-1", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})
	writeJobMeta(t, dir, "sprout-allinv", "job-inv-1", "UPUBKEY_TESTER")

	summaries, err := store.ListAllJobs(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job, got %d", len(summaries))
	}
	if summaries[0].InvokedBy != "UPUBKEY_TESTER" {
		t.Errorf("expected UPUBKEY_TESTER, got %s", summaries[0].InvokedBy)
	}
}

func TestDefaultCLIStorePath(t *testing.T) {
	path, err := DefaultCLIStorePath()
	if err != nil {
		t.Fatalf("DefaultCLIStorePath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	// Should end with "grlx/jobs".
	if filepath.Base(path) != "jobs" {
		t.Errorf("expected path ending with 'jobs', got %q", path)
	}
}

func TestCLIStore_RecordJobStart_ExistingJsonl(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Pre-create the JSONL file.
	sproutDir := filepath.Join(dir, "sprout-pre")
	os.MkdirAll(sproutDir, 0o700)
	jobFile := filepath.Join(sproutDir, "pre-job.jsonl")
	os.WriteFile(jobFile, []byte("existing content\n"), 0o600)

	meta := CLIJobMeta{
		JID:      "pre-job",
		SproutID: "sprout-pre",
		UserKey:  "UTEST",
	}

	// Should not overwrite existing JSONL.
	if err := store.RecordJobStart(meta); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(jobFile)
	if string(content) != "existing content\n" {
		t.Error("expected existing JSONL content to be preserved")
	}
}

func TestCLIStore_GetJob_WithBadMeta(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a job with valid JSONL but malformed meta.
	sproutDir := filepath.Join(dir, "sprout-badmeta")
	os.MkdirAll(sproutDir, 0o700)

	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	b, _ := json.Marshal(step)
	os.WriteFile(filepath.Join(sproutDir, "bad-meta-job.jsonl"), append(b, '\n'), 0o600)
	os.WriteFile(filepath.Join(sproutDir, "bad-meta-job.meta.json"), []byte("not json"), 0o600)

	summary, meta, err := store.GetJob("bad-meta-job")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if meta != nil {
		t.Error("expected nil meta for malformed meta file")
	}
}

func TestReap_RemovesMetaFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-meta-reap")
	os.MkdirAll(sproutDir, 0o700)

	// Create old job + meta.
	oldJob := filepath.Join(sproutDir, "old-with-meta.jsonl")
	oldMeta := filepath.Join(sproutDir, "old-with-meta.meta.json")
	os.WriteFile(oldJob, []byte("{}\n"), 0o644)
	os.WriteFile(oldMeta, []byte(`{"jid":"old-with-meta"}`), 0o644)

	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldJob, past, past)

	store.reap(24 * time.Hour)

	if _, err := os.Stat(oldJob); !os.IsNotExist(err) {
		t.Error("expected old job to be removed")
	}
	if _, err := os.Stat(oldMeta); !os.IsNotExist(err) {
		t.Error("expected old meta to be removed along with job")
	}
}

func TestListSproutDirsUnlocked_EmptyLogDir(t *testing.T) {
	store := NewStoreWithDir("")
	_, err := store.listSproutDirsUnlocked()
	if err != ErrInvalidJobDir {
		t.Errorf("expected ErrInvalidJobDir, got %v", err)
	}
}

func TestListSproutDirsUnlocked_NonexistentDir(t *testing.T) {
	store := NewStoreWithDir("/nonexistent/path/that/should/not/exist")
	sprouts, err := store.listSproutDirsUnlocked()
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
	if sprouts != nil {
		t.Errorf("expected nil sprouts, got %v", sprouts)
	}
}

func TestReap_SkipsNonJsonl(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-skip")
	os.MkdirAll(sproutDir, 0o700)

	// Create an old non-jsonl file — should NOT be removed.
	oldTxt := filepath.Join(sproutDir, "notes.txt")
	os.WriteFile(oldTxt, []byte("keep me"), 0o644)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldTxt, past, past)

	// Create a new jsonl file to keep sprout dir alive.
	newJob := filepath.Join(sproutDir, "new.jsonl")
	os.WriteFile(newJob, []byte("{}\n"), 0o644)

	store.reap(24 * time.Hour)

	if _, err := os.Stat(oldTxt); err != nil {
		t.Error("expected non-jsonl file to be preserved")
	}
}

func TestCLIStore_ListJobs_SortOrder(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second)

	// Create jobs with different start times.
	for i, offset := range []time.Duration{0, 10 * time.Second, 5 * time.Second} {
		jid := "sort-job-" + string(rune('a'+i))
		meta := CLIJobMeta{JID: jid, SproutID: "sprout-sort", UserKey: "U1", CreatedAt: time.Now()}
		store.RecordJobStart(meta)
		step := cook.StepCompletion{
			ID:               cook.StepID("s1"),
			CompletionStatus: cook.StepCompleted,
			Started:          now.Add(offset),
			Duration:         time.Second,
		}
		store.AppendStep("sprout-sort", jid, step)
	}

	jobs, err := store.ListJobs(0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3, got %d", len(jobs))
	}
	// Most recent first: sort-job-b (10s), sort-job-c (5s), sort-job-a (0s).
	if jobs[0].JID != "sort-job-b" {
		t.Errorf("expected sort-job-b first, got %s", jobs[0].JID)
	}
}
