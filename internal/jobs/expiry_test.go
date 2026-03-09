package jobs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReapRemovesExpiredJobs(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	// Create a sprout dir with two job files.
	sproutDir := filepath.Join(dir, "sprout-a")
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(sproutDir, "old-job.jsonl")
	newFile := filepath.Join(sproutDir, "new-job.jsonl")

	// Write dummy content.
	os.WriteFile(oldFile, []byte(`{}`+"\n"), 0o644)
	os.WriteFile(newFile, []byte(`{}`+"\n"), 0o644)

	// Backdate the old file.
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldFile, past, past)

	// Reap with a 24h TTL — old-job should be removed.
	store.reap(24 * time.Hour)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old-job.jsonl to be removed, but it still exists")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Errorf("expected new-job.jsonl to still exist, got error: %v", err)
	}
}

func TestReapRemovesEmptySproutDirs(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-b")
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(sproutDir, "only-job.jsonl")
	os.WriteFile(oldFile, []byte(`{}`+"\n"), 0o644)

	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldFile, past, past)

	store.reap(24 * time.Hour)

	if _, err := os.Stat(sproutDir); !os.IsNotExist(err) {
		t.Errorf("expected empty sprout dir to be removed")
	}
}

func TestReapZeroTTLNoOp(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-c")
	os.MkdirAll(sproutDir, 0o700)
	jobFile := filepath.Join(sproutDir, "job.jsonl")
	os.WriteFile(jobFile, []byte(`{}`+"\n"), 0o644)

	past := time.Now().Add(-9999 * time.Hour)
	os.Chtimes(jobFile, past, past)

	// StartReaper with 0 should not start a goroutine, but we can call reap
	// with 0 directly — it should still not delete (TTL=0 means keep forever
	// at the StartReaper level, but reap itself treats cutoff as now).
	// Since StartReaper bails on ttl<=0, this test just verifies StartReaper
	// doesn't panic.
	store.StartReaper(0)

	if _, err := os.Stat(jobFile); err != nil {
		t.Errorf("expected job file to still exist when TTL=0")
	}
}
