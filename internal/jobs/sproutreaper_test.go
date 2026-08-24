package jobs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReapFlatDirRemovesExpiredJobs(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "old-job.jsonl")
	oldMetaFile := filepath.Join(dir, "old-job.meta.json")
	newFile := filepath.Join(dir, "new-job.jsonl")
	newMetaFile := filepath.Join(dir, "new-job.meta.json")

	if err := os.WriteFile(oldFile, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldMetaFile, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newMetaFile, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Backdate the old file so it falls outside the TTL window.
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldMetaFile, past, past); err != nil {
		t.Fatal(err)
	}

	reapFlatDir(dir, 24*time.Hour)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old-job.jsonl to be removed, but it still exists")
	}
	if _, err := os.Stat(oldMetaFile); !os.IsNotExist(err) {
		t.Errorf("expected old-job.meta.json to be removed, but it still exists")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Errorf("expected new-job.jsonl to still exist, got error: %v", err)
	}
	if _, err := os.Stat(newMetaFile); err != nil {
		t.Errorf("expected new-job.meta.json to still exist, got error: %v", err)
	}
}

func TestReapFlatDirIgnoresSubdirsAndNonJSONL(t *testing.T) {
	dir := t.TempDir()

	// A subdirectory (farmer-style layout) must not be touched or descended.
	subDir := filepath.Join(dir, "sprout-a")
	if err := os.MkdirAll(subDir, 0o700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(subDir, "job.jsonl")
	if err := os.WriteFile(nested, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A non-.jsonl file must be left alone even if old.
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(nested, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(other, past, past); err != nil {
		t.Fatal(err)
	}

	reapFlatDir(dir, 24*time.Hour)

	if _, err := os.Stat(nested); err != nil {
		t.Errorf("expected nested subdir job to be untouched, got: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("expected non-jsonl file to be untouched, got: %v", err)
	}
}

func TestStartSproutReaperZeroTTLNoOp(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "job.jsonl")
	if err := os.WriteFile(jobFile, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-9999 * time.Hour)
	if err := os.Chtimes(jobFile, past, past); err != nil {
		t.Fatal(err)
	}

	// ttl <= 0 disables the reaper entirely; the file must survive.
	StartSproutReaper(context.Background(), dir, 0)

	if _, err := os.Stat(jobFile); err != nil {
		t.Errorf("expected job file to still exist when TTL=0, got: %v", err)
	}
}

func TestStartSproutReaperReapsOnStartupAndStopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	// A pre-expired file lets us confirm the immediate startup reap ran.
	jobFile := filepath.Join(dir, "stale.jsonl")
	if err := os.WriteFile(jobFile, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(jobFile, past, past); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartSproutReaper(ctx, dir, time.Hour)

	// The reaper runs one reap immediately on startup; poll until the stale
	// file is gone, which proves the goroutine is alive and doing work.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(jobFile); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reaper did not remove the stale file on startup")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancelling must let the goroutine exit without panicking.
	cancel()
}
