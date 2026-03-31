package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
)

func writeJobFile(t *testing.T, dir, sproutID, jid string, steps []cook.StepCompletion) {
	t.Helper()
	sproutDir := filepath.Join(dir, sproutID)
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		t.Fatal(err)
	}
	jobFile := filepath.Join(sproutDir, fmt.Sprintf("%s.jsonl", jid))
	f, err := os.Create(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, step := range steps {
		b, marshalErr := json.Marshal(step)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		fmt.Fprintln(f, string(b))
	}
}

func makeStep(id string, status cook.CompletionStatus, started time.Time, duration time.Duration) cook.StepCompletion {
	return cook.StepCompletion{
		ID:               cook.StepID(id),
		CompletionStatus: status,
		Started:          started,
		Duration:         duration,
	}
}

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	if store.logDir != dir {
		t.Errorf("expected logDir %q, got %q", dir, store.logDir)
	}
}

func TestGetJob_Found(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepCompleted, now, 5*time.Second),
		makeStep("step-2", cook.StepCompleted, now.Add(5*time.Second), 3*time.Second),
	}
	writeJobFile(t, dir, "sprout-a", "job-123", steps)

	summary, err := store.GetJob("sprout-a", "job-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.JID != "job-123" {
		t.Errorf("expected JID job-123, got %s", summary.JID)
	}
	if summary.SproutID != "sprout-a" {
		t.Errorf("expected SproutID sprout-a, got %s", summary.SproutID)
	}
	if summary.Total != 2 {
		t.Errorf("expected 2 total steps, got %d", summary.Total)
	}
	if summary.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", summary.Succeeded)
	}
	if summary.Status != JobSucceeded {
		t.Errorf("expected status succeeded, got %s", summary.Status)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	_, err := store.GetJob("nonexistent", "no-such-job")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestFindJob(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepFailed, now, 2*time.Second),
	}
	writeJobFile(t, dir, "sprout-b", "unique-jid", steps)

	summary, err := store.FindJob("unique-jid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.SproutID != "sprout-b" {
		t.Errorf("expected sprout-b, got %s", summary.SproutID)
	}
	if summary.Status != JobFailed {
		t.Errorf("expected status failed, got %s", summary.Status)
	}
}

func TestFindJob_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	_, err := store.FindJob("missing")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestListJobsForSprout(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	writeJobFile(t, dir, "sprout-c", "job-1", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})
	writeJobFile(t, dir, "sprout-c", "job-2", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now.Add(10*time.Second), time.Second),
	})
	writeJobFile(t, dir, "sprout-c", "job-3", []cook.StepCompletion{
		makeStep("s1", cook.StepFailed, now.Add(20*time.Second), time.Second),
	})

	summaries, err := store.ListJobsForSprout("sprout-c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(summaries))
	}
	// Should be sorted by start time, most recent first
	if summaries[0].JID != "job-3" {
		t.Errorf("expected job-3 first (most recent), got %s", summaries[0].JID)
	}
	if summaries[2].JID != "job-1" {
		t.Errorf("expected job-1 last (oldest), got %s", summaries[2].JID)
	}
}

func TestListJobsForSprout_NoJobs(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	_, err := store.ListJobsForSprout("nonexistent")
	if err != ErrSproutNoJobs {
		t.Errorf("expected ErrSproutNoJobs, got %v", err)
	}
}

func TestListAllJobs(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	writeJobFile(t, dir, "sprout-x", "job-a", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})
	writeJobFile(t, dir, "sprout-y", "job-b", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now.Add(5*time.Second), time.Second),
	})

	summaries, err := store.ListAllJobs(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(summaries))
	}
	// Most recent first
	if summaries[0].JID != "job-b" {
		t.Errorf("expected job-b first, got %s", summaries[0].JID)
	}
}

func TestListAllJobs_WithLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	for i := range 5 {
		writeJobFile(t, dir, "sprout-z", fmt.Sprintf("job-%d", i), []cook.StepCompletion{
			makeStep("s1", cook.StepCompleted, now.Add(time.Duration(i)*time.Minute), time.Second),
		})
	}

	summaries, err := store.ListAllJobs(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 3 {
		t.Errorf("expected 3 jobs (limited), got %d", len(summaries))
	}
}

func TestListSprouts(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	// Create sprout directories
	for _, sprout := range []string{"alpha", "beta", "gamma"} {
		if err := os.MkdirAll(filepath.Join(dir, sprout), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	sprouts, err := store.ListSprouts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprouts) != 3 {
		t.Errorf("expected 3 sprouts, got %d", len(sprouts))
	}
}

func TestCountJobsForSprout(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	writeJobFile(t, dir, "sprout-count", "j1", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})
	writeJobFile(t, dir, "sprout-count", "j2", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})

	count, err := store.CountJobsForSprout("sprout-count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCountJobsForSprout_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	count, err := store.CountJobsForSprout("ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestDetermineJobStatus(t *testing.T) {
	tests := []struct {
		name     string
		steps    []cook.StepCompletion
		expected JobStatus
	}{
		{
			name:     "empty steps returns pending",
			steps:    nil,
			expected: JobPending,
		},
		{
			name: "all completed returns succeeded",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepCompleted, time.Now(), time.Second),
				makeStep("s2", cook.StepCompleted, time.Now(), time.Second),
			},
			expected: JobSucceeded,
		},
		{
			name: "all skipped returns succeeded",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepSkipped, time.Now(), 0),
			},
			expected: JobSucceeded,
		},
		{
			name: "any in progress returns running",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepCompleted, time.Now(), time.Second),
				makeStep("s2", cook.StepInProgress, time.Now(), 0),
			},
			expected: JobRunning,
		},
		{
			name: "any failed returns failed",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepCompleted, time.Now(), time.Second),
				makeStep("s2", cook.StepFailed, time.Now(), time.Second),
			},
			expected: JobFailed,
		},
		{
			name: "mix of completed and not started returns partial",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepCompleted, time.Now(), time.Second),
				makeStep("s2", cook.StepNotStarted, time.Time{}, 0),
			},
			expected: JobPartial,
		},
		{
			name: "all not started returns pending",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepNotStarted, time.Time{}, 0),
				makeStep("s2", cook.StepNotStarted, time.Time{}, 0),
			},
			expected: JobPending,
		},
		{
			name: "in progress takes priority over failed",
			steps: []cook.StepCompletion{
				makeStep("s1", cook.StepFailed, time.Now(), time.Second),
				makeStep("s2", cook.StepInProgress, time.Now(), 0),
			},
			expected: JobRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineJobStatus(tt.steps)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestJobStatusJSON(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected string
	}{
		{JobPending, `"pending"`},
		{JobRunning, `"running"`},
		{JobSucceeded, `"succeeded"`},
		{JobFailed, `"failed"`},
		{JobPartial, `"partial"`},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			b, err := json.Marshal(tt.status)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			if string(b) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(b))
			}

			var unmarshaled JobStatus
			if err := json.Unmarshal(b, &unmarshaled); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if unmarshaled != tt.status {
				t.Errorf("expected %v after roundtrip, got %v", tt.status, unmarshaled)
			}
		})
	}
}

func TestBuildSummary_Duration(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	steps := []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, 5*time.Second),
		makeStep("s2", cook.StepCompleted, now.Add(5*time.Second), 10*time.Second),
		makeStep("s3", cook.StepFailed, now.Add(2*time.Second), 3*time.Second),
	}

	summary := buildSummary("test-jid", "test-sprout", steps)

	if !summary.StartedAt.Equal(now) {
		t.Errorf("expected StartedAt %v, got %v", now, summary.StartedAt)
	}
	// Latest end: s2 started at +5s with 10s duration = +15s from now
	expectedDuration := 15 * time.Second
	if summary.Duration != expectedDuration {
		t.Errorf("expected duration %v, got %v", expectedDuration, summary.Duration)
	}
	if summary.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", summary.Succeeded)
	}
	if summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", summary.Failed)
	}
}

func TestReadJobFile_EmptyLines(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "test.jsonl")

	step := makeStep("s1", cook.StepCompleted, time.Now().Truncate(time.Second), time.Second)
	b, _ := json.Marshal(step)
	// File with blank lines
	content := fmt.Sprintf("\n%s\n\n%s\n\n", string(b), string(b))
	if err := os.WriteFile(jobFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps (skipping blank lines), got %d", len(steps))
	}
}

func TestListAllJobs_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	summaries, err := store.ListAllJobs(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(summaries))
	}
}

func TestListSprouts_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sprouts, err := store.ListSprouts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprouts) != 0 {
		t.Errorf("expected 0 sprouts, got %d", len(sprouts))
	}
}

func TestListSprouts_NonexistentDir(t *testing.T) {
	store := NewStoreWithDir("/nonexistent/path/that/does/not/exist")

	sprouts, err := store.ListSprouts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sprouts != nil {
		t.Errorf("expected nil sprouts, got %v", sprouts)
	}
}

func writeJobMeta(t *testing.T, dir, sproutID, jid, invokedBy string) {
	t.Helper()
	sproutDir := filepath.Join(dir, sproutID)
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		t.Fatal(err)
	}
	meta := JobMeta{
		JID:       jid,
		InvokedBy: invokedBy,
		CreatedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	metaFile := filepath.Join(sproutDir, fmt.Sprintf("%s.meta.json", jid))
	if err := os.WriteFile(metaFile, data, 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestGetJob_WithInvokedBy(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now()

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepCompleted, now, time.Second),
	}
	writeJobFile(t, dir, "web-1", "job-meta-1", steps)
	writeJobMeta(t, dir, "web-1", "job-meta-1", "UPUBKEY_ALICE")

	summary, err := store.GetJob("web-1", "job-meta-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary.InvokedBy != "UPUBKEY_ALICE" {
		t.Errorf("InvokedBy = %q, want UPUBKEY_ALICE", summary.InvokedBy)
	}
}

func TestGetJob_WithoutMeta(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now()

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepCompleted, now, time.Second),
	}
	writeJobFile(t, dir, "web-1", "job-no-meta", steps)

	summary, err := store.GetJob("web-1", "job-no-meta")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary.InvokedBy != "" {
		t.Errorf("InvokedBy = %q, want empty", summary.InvokedBy)
	}
}

func TestFindJob_WithInvokedBy(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now()

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepCompleted, now, time.Second),
	}
	writeJobFile(t, dir, "db-1", "job-find-meta", steps)
	writeJobMeta(t, dir, "db-1", "job-find-meta", "UPUBKEY_BOB")

	summary, err := store.FindJob("job-find-meta")
	if err != nil {
		t.Fatalf("FindJob: %v", err)
	}
	if summary.InvokedBy != "UPUBKEY_BOB" {
		t.Errorf("InvokedBy = %q, want UPUBKEY_BOB", summary.InvokedBy)
	}
}

func TestListJobsForSprout_WithInvokedBy(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now()

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepCompleted, now, time.Second),
	}
	writeJobFile(t, dir, "app-1", "job-list-1", steps)
	writeJobMeta(t, dir, "app-1", "job-list-1", "UPUBKEY_CAROL")
	writeJobFile(t, dir, "app-1", "job-list-2", steps)
	// No meta for job-list-2

	summaries, err := store.ListJobsForSprout("app-1")
	if err != nil {
		t.Fatalf("ListJobsForSprout: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	foundMeta := false
	foundNoMeta := false
	for _, s := range summaries {
		if s.JID == "job-list-1" && s.InvokedBy == "UPUBKEY_CAROL" {
			foundMeta = true
		}
		if s.JID == "job-list-2" && s.InvokedBy == "" {
			foundNoMeta = true
		}
	}
	if !foundMeta {
		t.Error("job-list-1 should have InvokedBy=UPUBKEY_CAROL")
	}
	if !foundNoMeta {
		t.Error("job-list-2 should have empty InvokedBy")
	}
}

func TestDeleteJob_Found(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	steps := []cook.StepCompletion{
		makeStep("step-1", cook.StepCompleted, now, 2*time.Second),
	}
	writeJobFile(t, dir, "sprout-del", "del-job-1", steps)

	// Also write a meta file.
	sproutDir := filepath.Join(dir, "sprout-del")
	metaFile := filepath.Join(sproutDir, "del-job-1.meta.json")
	os.WriteFile(metaFile, []byte(`{"invoked_by":"testuser"}`), 0o640)

	// Confirm it exists.
	_, err := store.FindJob("del-job-1")
	if err != nil {
		t.Fatalf("setup: job should exist: %v", err)
	}

	// Delete it.
	if err := store.DeleteJob("del-job-1"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// Confirm it's gone.
	_, err = store.FindJob("del-job-1")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound after delete, got %v", err)
	}

	// Confirm meta file is also gone.
	if _, statErr := os.Stat(metaFile); !os.IsNotExist(statErr) {
		t.Error("meta file should have been deleted")
	}
}

func TestDeleteJob_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	err := store.DeleteJob("nonexistent-jid")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestDeleteJob_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	err := store.DeleteJob("any-jid")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound for empty dir, got %v", err)
	}
}

func TestReadJobMeta_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	sproutDir := filepath.Join(dir, "sprout-bad")
	os.MkdirAll(sproutDir, 0o700)

	metaFile := filepath.Join(sproutDir, "bad-job.meta.json")
	os.WriteFile(metaFile, []byte("not json"), 0o640)

	got := readJobMeta(sproutDir, "bad-job")
	if got != "" {
		t.Errorf("readJobMeta = %q, want empty for malformed JSON", got)
	}
}
