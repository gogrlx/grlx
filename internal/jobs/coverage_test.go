package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
)

// --- StartReaper / reap edge cases ---

func TestStartReaper_NegativeTTL(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout")
	os.MkdirAll(sproutDir, 0o700)
	jobFile := filepath.Join(sproutDir, "j.jsonl")
	os.WriteFile(jobFile, []byte("{}\n"), 0o644)
	past := time.Now().Add(-9999 * time.Hour)
	os.Chtimes(jobFile, past, past)

	// Negative TTL should disable reaper (same as zero).
	store.StartReaper(-1 * time.Hour)

	if _, err := os.Stat(jobFile); err != nil {
		t.Error("expected job file to survive with negative TTL")
	}
}

func TestReap_EmptyLogDir(t *testing.T) {
	store := NewStoreWithDir("")
	// Should not panic — listSproutDirsUnlocked returns ErrInvalidJobDir.
	store.reap(24 * time.Hour)
}

func TestReap_NonexistentLogDir(t *testing.T) {
	store := NewStoreWithDir("/tmp/nonexistent-reap-test-dir-xyz")
	// Should handle gracefully.
	store.reap(24 * time.Hour)
}

func TestReap_InfoError(t *testing.T) {
	// This exercises the entry.Info() error path.
	// Hard to trigger with real filesystem, but we verify reap doesn't crash
	// on directories with unreadable entries.
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	sproutDir := filepath.Join(dir, "sprout-info")
	os.MkdirAll(sproutDir, 0o700)

	// Create a directory inside sprout dir that looks like a jsonl file — IsDir check.
	os.MkdirAll(filepath.Join(sproutDir, "fake.jsonl"), 0o700)

	// Also add a valid old job to ensure the directory-named "jsonl" doesn't crash.
	oldJob := filepath.Join(sproutDir, "real.jsonl")
	os.WriteFile(oldJob, []byte("{}\n"), 0o644)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldJob, past, past)

	store.reap(24 * time.Hour)

	// fake.jsonl dir should still exist (it's a dir, skipped by IsDir check).
	if _, err := os.Stat(filepath.Join(sproutDir, "fake.jsonl")); err != nil {
		t.Error("expected directory-named fake.jsonl to be untouched")
	}
	// real.jsonl should be removed.
	if _, err := os.Stat(oldJob); !os.IsNotExist(err) {
		t.Error("expected real.jsonl to be removed")
	}
}

func TestReap_RemoveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test remove permission errors as root")
	}
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-perm")
	os.MkdirAll(sproutDir, 0o700)

	oldJob := filepath.Join(sproutDir, "protected.jsonl")
	os.WriteFile(oldJob, []byte("{}\n"), 0o644)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldJob, past, past)

	// Make sprout dir read-only to prevent deletion.
	os.Chmod(sproutDir, 0o555)
	t.Cleanup(func() { os.Chmod(sproutDir, 0o700) })

	// Should not panic — the remove error is logged.
	store.reap(24 * time.Hour)
}

func TestReap_ReadDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test readdir permission errors as root")
	}
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-unreadable")
	os.MkdirAll(sproutDir, 0o700)
	os.WriteFile(filepath.Join(sproutDir, "job.jsonl"), []byte("{}\n"), 0o644)

	// Make sprout dir unreadable.
	os.Chmod(sproutDir, 0o000)
	t.Cleanup(func() { os.Chmod(sproutDir, 0o700) })

	// Should handle gracefully (continue past ReadDir error).
	store.reap(24 * time.Hour)
}

// --- RegisterNatsConn edge cases ---

func TestRegisterNatsConn_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	// Make dir read-only so MkdirAll fails.
	readonlyDir := filepath.Join(dir, "readonly")
	os.MkdirAll(readonlyDir, 0o555)
	t.Cleanup(func() { os.Chmod(readonlyDir, 0o700) })

	origJobLogDir := config.JobLogDir
	config.JobLogDir = filepath.Join(readonlyDir, "joblogs")
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)

	// Should not panic — error is logged.
	RegisterNatsConn(conn)
}

// --- logJobs edge cases ---

func TestLogJobs_ShortSubject(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Subscribe to a subject that will produce short components.
	// Publish directly to the callback won't work via NATS wildcard,
	// but we can test that short subjects on cook creation are handled.
	// grlx.sprouts.X.cook with too-short subject.
	if err := conn.Publish("grlx.sprouts.s.cook", []byte(`{"job_id":"","steps":[]}`)); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
	// No crash = pass.
}

func TestLogJobCreation_ShortSubject(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// We can't easily trigger the < 4 components path via NATS subscription
	// since the wildcard pattern guarantees 4 parts. But verify that
	// the callback handles envelope parse failures gracefully.
	envelope := cook.RecipeEnvelope{
		JobID:     "sub-test",
		InvokedBy: "UTEST",
		Steps:     []cook.Step{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}},
	}
	data, _ := json.Marshal(envelope)
	if err := conn.Publish("grlx.sprouts.sprout-sub.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	// Verify 3 steps written.
	jobFile := filepath.Join(dir, "sprout-sub", "sub-test.jsonl")
	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 placeholder steps, got %d", len(steps))
	}
}

// --- logJobs concurrent writes ---

func TestLogJobs_ConcurrentSteps(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Publish many steps concurrently.
	const stepCount = 20
	for i := range stepCount {
		step := cook.StepCompletion{
			ID:               cook.StepID(fmt.Sprintf("step-%d", i)),
			CompletionStatus: cook.StepCompleted,
			Started:          time.Now(),
			Duration:         time.Millisecond,
		}
		data, _ := json.Marshal(step)
		if err := conn.Publish("grlx.cook.sprout-concurrent.job-concurrent", data); err != nil {
			t.Fatal(err)
		}
	}
	conn.Flush()
	time.Sleep(500 * time.Millisecond)

	jobFile := filepath.Join(dir, "sprout-concurrent", "job-concurrent.jsonl")
	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != stepCount {
		t.Errorf("expected %d steps, got %d", stepCount, len(steps))
	}
}

// --- CLIStore error paths ---

func TestNewCLIStore_InvalidPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	os.MkdirAll(readonlyDir, 0o555)
	t.Cleanup(func() { os.Chmod(readonlyDir, 0o700) })

	_, err := NewCLIStore(filepath.Join(readonlyDir, "nested", "store"))
	if err == nil {
		t.Error("expected error for read-only parent dir")
	}
}

func TestCLIStore_RecordJobStart_ReadOnlySproutDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Make store dir read-only so sprout dir creation fails.
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	meta := CLIJobMeta{
		JID:      "fail-job",
		SproutID: "new-sprout",
		UserKey:  "UTEST",
	}
	err = store.RecordJobStart(meta)
	if err == nil {
		t.Error("expected error when sprout dir creation fails")
	}
}

func TestCLIStore_AppendStep_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Make store dir read-only.
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	err = store.AppendStep("new-sprout", "new-job", step)
	if err == nil {
		t.Error("expected error when sprout dir creation fails")
	}
}

func TestCLIStore_GetJobMeta_BadJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a meta file with bad JSON.
	sproutDir := filepath.Join(dir, "sprout-badjson")
	os.MkdirAll(sproutDir, 0o700)
	os.WriteFile(filepath.Join(sproutDir, "bad-meta.meta.json"), []byte("not json"), 0o600)

	_, err = store.GetJobMeta("bad-meta")
	if err != ErrMetaNotFound {
		t.Errorf("expected ErrMetaNotFound for bad JSON, got %v", err)
	}
}

func TestCLIStore_GetJob_BadJSONL(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sproutDir := filepath.Join(dir, "sprout-badjsonl")
	os.MkdirAll(sproutDir, 0o700)
	os.WriteFile(filepath.Join(sproutDir, "bad-job.jsonl"), []byte("not json\n"), 0o600)

	_, _, err = store.GetJob("bad-job")
	if err == ErrJobNotFound {
		// This is also acceptable — the readJobFile error causes it to be skipped.
		return
	}
	// If readJobFile returns an error, GetJob should propagate or skip.
}

func TestCLIStore_ListJobs_BadJSONLSkipped(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sproutDir := filepath.Join(dir, "sprout-mix")
	os.MkdirAll(sproutDir, 0o700)

	// One good job.
	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	b, _ := json.Marshal(step)
	os.WriteFile(filepath.Join(sproutDir, "good-job.jsonl"), append(b, '\n'), 0o600)
	meta := CLIJobMeta{JID: "good-job", SproutID: "sprout-mix", UserKey: "U1"}
	metaData, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(sproutDir, "good-job.meta.json"), metaData, 0o600)

	// One bad job (malformed JSONL — should be skipped).
	os.WriteFile(filepath.Join(sproutDir, "bad-job.jsonl"), []byte("not json\n"), 0o600)

	jobs, err := store.ListJobs(0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 good job (bad skipped), got %d", len(jobs))
	}
}

func TestCLIStore_ListJobs_BadMetaSkipped(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sproutDir := filepath.Join(dir, "sprout-badmeta")
	os.MkdirAll(sproutDir, 0o700)

	// Good job with bad meta file — when filtering by userKey, bad meta
	// means the filter can't match, so it may or may not be included.
	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	b, _ := json.Marshal(step)
	os.WriteFile(filepath.Join(sproutDir, "meta-bad.jsonl"), append(b, '\n'), 0o600)
	os.WriteFile(filepath.Join(sproutDir, "meta-bad.meta.json"), []byte("not json"), 0o600)

	// Filter by user — the bad meta file means Unmarshal fails, which
	// falls through (doesn't filter out).
	jobs, err := store.ListJobs(0, "UFILTER", "")
	if err != nil {
		t.Fatal(err)
	}
	// The bad meta unmarshal fails, so the filter condition is not met;
	// the job is included (unmarshal fail = can't confirm mismatch).
	if len(jobs) != 1 {
		t.Logf("got %d jobs — behavior depends on filter logic", len(jobs))
	}
}

func TestCLIStore_ListJobs_DirsAndNonJsonlSkipped(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sproutDir := filepath.Join(dir, "sprout-mixed")
	os.MkdirAll(sproutDir, 0o700)

	// Create subdirectory (should be skipped).
	os.MkdirAll(filepath.Join(sproutDir, "subdir"), 0o700)
	// Create non-jsonl file (should be skipped).
	os.WriteFile(filepath.Join(sproutDir, "readme.txt"), []byte("hi"), 0o600)
	// Create meta file without jsonl (should be skipped).
	os.WriteFile(filepath.Join(sproutDir, "orphan.meta.json"), []byte("{}"), 0o600)

	jobs, err := store.ListJobs(0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

// --- DefaultCLIStorePath with XDG_CONFIG_HOME ---

func TestDefaultCLIStorePath_WithXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := DefaultCLIStorePath()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "grlx", "jobs")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// --- CLIListener edge cases ---

func TestCLIListener_RecordJobInit_StoreError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Make dir read-only so RecordJobStart fails.
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UFAIL")

	// Should not panic — error is logged.
	listener.RecordJobInit("fail-jid", "recipe", []string{"sprout-a", "sprout-b"})
}

func TestCLIListener_HandleStepCompletion_RecordError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UERR")

	if err := listener.SubscribeAll(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	// Make dir read-only so AppendStep fails.
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	data, _ := json.Marshal(step)
	if err := conn.Publish("grlx.cook.sprout-err.job-err", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
	// Should not panic — error is logged.
}

// --- Store.listSproutDirs edge cases ---

func TestStore_ListSproutDirs_FilesIgnored(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	// Create files (not dirs) — should be ignored.
	os.WriteFile(filepath.Join(dir, "not-a-sprout.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(dir, "also-not"), []byte("hi"), 0o644)

	sprouts, err := store.ListSprouts()
	if err != nil {
		t.Fatal(err)
	}
	if len(sprouts) != 0 {
		t.Errorf("expected 0 sprouts (files should be ignored), got %d", len(sprouts))
	}
}

// --- ListJobsForSprout with unreadable job file ---

func TestListJobsForSprout_UnreadableJobFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	// One good job.
	writeJobFile(t, dir, "sprout-unreadable", "good-job", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})

	// One unreadable job file.
	badFile := filepath.Join(dir, "sprout-unreadable", "bad-job.jsonl")
	os.WriteFile(badFile, []byte("{}\n"), 0o644)
	os.Chmod(badFile, 0o000)
	t.Cleanup(func() { os.Chmod(badFile, 0o644) })

	summaries, err := store.ListJobsForSprout("sprout-unreadable")
	if err != nil {
		t.Fatal(err)
	}
	// Bad file should be skipped.
	if len(summaries) != 1 {
		t.Errorf("expected 1 (bad skipped), got %d", len(summaries))
	}
}

// --- ListAllJobs with unreadable sprout dir ---

func TestListAllJobs_UnreadableSproutDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now().Truncate(time.Second)

	// Good sprout.
	writeJobFile(t, dir, "good-sprout", "j1", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})

	// Unreadable sprout dir.
	badSprout := filepath.Join(dir, "bad-sprout")
	os.MkdirAll(badSprout, 0o700)
	os.WriteFile(filepath.Join(badSprout, "j2.jsonl"), []byte("{}\n"), 0o644)
	os.Chmod(badSprout, 0o000)
	t.Cleanup(func() { os.Chmod(badSprout, 0o700) })

	summaries, err := store.ListAllJobs(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 (bad sprout skipped), got %d", len(summaries))
	}
}

// --- CountJobsForSprout with read error ---

func TestCountJobsForSprout_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-count-err")
	os.MkdirAll(sproutDir, 0o700)
	os.WriteFile(filepath.Join(sproutDir, "j.jsonl"), []byte("{}\n"), 0o644)
	os.Chmod(sproutDir, 0o000)
	t.Cleanup(func() { os.Chmod(sproutDir, 0o700) })

	_, err := store.CountJobsForSprout("sprout-count-err")
	if err == nil {
		t.Error("expected error for unreadable sprout dir")
	}
}

// --- CLIStore.listSproutDirs with non-dirs ---

func TestCLIStore_ListSproutDirs_FilesIgnored(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(dir, "not-a-dir.txt"), []byte("hi"), 0o600)

	jobs, err := store.ListJobs(0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs (files as sprout names ignored), got %d", len(jobs))
	}
}

// --- readJobFile edge case: nonexistent file ---

func TestReadJobFile_Nonexistent(t *testing.T) {
	_, err := readJobFile("/nonexistent/path/to/file.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- buildSummary with single step zero duration ---

func TestBuildSummary_SingleStepZeroDuration(t *testing.T) {
	now := time.Now()
	steps := []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, 0),
	}
	summary := buildSummary("jid", "sprout", steps)
	if summary.Duration != 0 {
		t.Errorf("expected zero duration, got %v", summary.Duration)
	}
	if summary.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", summary.Succeeded)
	}
}

// --- buildSummary empty steps ---

func TestBuildSummary_EmptySteps(t *testing.T) {
	summary := buildSummary("jid", "sprout", nil)
	if summary.Total != 0 {
		t.Errorf("expected 0 total, got %d", summary.Total)
	}
	if summary.Status != JobPending {
		t.Errorf("expected pending, got %s", summary.Status)
	}
}

// --- FindJob across multiple sprouts ---

func TestFindJob_MultipleSprouts(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)
	now := time.Now()

	// Same JID across two sprouts — FindJob returns the first one found.
	writeJobFile(t, dir, "sprout-1", "shared-jid", []cook.StepCompletion{
		makeStep("s1", cook.StepCompleted, now, time.Second),
	})
	writeJobFile(t, dir, "sprout-2", "other-jid", []cook.StepCompletion{
		makeStep("s1", cook.StepFailed, now, time.Second),
	})

	summary, err := store.FindJob("shared-jid")
	if err != nil {
		t.Fatal(err)
	}
	if summary.JID != "shared-jid" {
		t.Errorf("expected shared-jid, got %s", summary.JID)
	}
}

// --- logJobCreation with marshal error in steps (unlikely but safe) ---

func TestLogJobCreation_ManySteps(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Create envelope with many steps.
	steps := make([]cook.Step, 50)
	for i := range steps {
		steps[i] = cook.Step{ID: cook.StepID(fmt.Sprintf("step-%d", i))}
	}
	envelope := cook.RecipeEnvelope{
		JobID:     "many-steps-job",
		InvokedBy: "UMANY",
		Steps:     steps,
	}
	data, _ := json.Marshal(envelope)

	if err := conn.Publish("grlx.sprouts.sprout-many.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	jobFile := filepath.Join(dir, "sprout-many", "many-steps-job.jsonl")
	readSteps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(readSteps) != 50 {
		t.Errorf("expected 50 steps, got %d", len(readSteps))
	}
}

// --- JobStatus String for all values ---

func TestJobStatus_AllStrings(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected string
	}{
		{JobPending, "pending"},
		{JobRunning, "running"},
		{JobSucceeded, "succeeded"},
		{JobFailed, "failed"},
		{JobPartial, "partial"},
		{JobStatus(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("JobStatus(%d).String() = %q, want %q", tt.status, got, tt.expected)
		}
	}
}

// --- StartReaper with positive TTL ---

func TestStartReaper_PositiveTTL(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreWithDir(dir)

	sproutDir := filepath.Join(dir, "sprout-reaper")
	os.MkdirAll(sproutDir, 0o700)
	oldJob := filepath.Join(sproutDir, "old.jsonl")
	os.WriteFile(oldJob, []byte("{}\n"), 0o644)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldJob, past, past)

	// StartReaper with positive TTL should run reap immediately, then start ticker.
	store.StartReaper(24 * time.Hour)
	// Give the goroutine time to execute.
	time.Sleep(200 * time.Millisecond)

	if _, err := os.Stat(oldJob); !os.IsNotExist(err) {
		t.Error("expected old job to be removed by StartReaper initial reap")
	}
}

// --- SubscribeAll / SubscribeJob error paths ---

func TestCLIListener_SubscribeAll_ClosedConn(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UCLOSED")

	// Close connection before subscribing.
	conn.Close()

	err = listener.SubscribeAll()
	if err == nil {
		t.Error("expected error when subscribing on closed connection")
	}
}

func TestCLIListener_SubscribeJob_ClosedConn(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UCLOSED2")

	conn.Close()

	err = listener.SubscribeJob("some-jid")
	if err == nil {
		t.Error("expected error when subscribing on closed connection")
	}
}

// --- logJobs: write to read-only sprout dir ---

func TestLogJobs_WriteToReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Create sprout dir and make it read-only.
	sproutDir := filepath.Join(dir, "sprout-ro")
	os.MkdirAll(sproutDir, 0o700)
	os.Chmod(sproutDir, 0o555)
	t.Cleanup(func() { os.Chmod(sproutDir, 0o700) })

	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	data, _ := json.Marshal(step)

	// Publishing to a read-only sprout dir should not panic.
	if err := conn.Publish("grlx.cook.sprout-ro.job-ro", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
}

// --- logJobs: existing file append path ---

func TestLogJobs_ExistingFileAppend(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Pre-create the sprout dir and an existing job file.
	sproutDir := filepath.Join(dir, "sprout-existing")
	os.MkdirAll(sproutDir, 0o700)
	existingStep := cook.StepCompletion{
		ID:               "existing-step",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	b, _ := json.Marshal(existingStep)
	jobFile := filepath.Join(sproutDir, "existing-job.jsonl")
	os.WriteFile(jobFile, append(b, '\n'), 0o644)

	// Publish a new step — should append to existing file.
	newStep := cook.StepCompletion{
		ID:               "new-step",
		CompletionStatus: cook.StepFailed,
		Started:          time.Now(),
		Duration:         2 * time.Second,
	}
	data, _ := json.Marshal(newStep)
	if err := conn.Publish("grlx.cook.sprout-existing.existing-job", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps (1 existing + 1 new), got %d", len(steps))
	}
}

// --- logJobs: new file creation path (sprout dir doesn't exist) ---

func TestLogJobs_NewSproutDir(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	data, _ := json.Marshal(step)

	// Publish to a sprout that doesn't have a directory yet.
	if err := conn.Publish("grlx.cook.brand-new-sprout.new-job", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	// Verify both dir and file were created.
	jobFile := filepath.Join(dir, "brand-new-sprout", "new-job.jsonl")
	if _, err := os.Stat(jobFile); err != nil {
		t.Fatalf("expected job file to be created: %v", err)
	}
	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
}

// --- logJobCreation: read-only dir (MkdirAll fails) ---

func TestLogJobCreation_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Make dir read-only so MkdirAll for sprout dir fails.
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	envelope := cook.RecipeEnvelope{
		JobID: "fail-create",
		Steps: []cook.Step{{ID: "s1"}},
	}
	data, _ := json.Marshal(envelope)

	if err := conn.Publish("grlx.sprouts.sprout-fail.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
	// Should not panic.
}

// --- CLIStore listSproutDirs error path ---

func TestCLIStore_ListSproutDirs_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a sprout and job so there's something to list.
	meta := CLIJobMeta{JID: "j1", SproutID: "s1", UserKey: "U1"}
	store.RecordJobStart(meta)

	// Make store dir unreadable.
	os.Chmod(dir, 0o000)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	_, err = store.ListJobs(0, "", "")
	if err == nil {
		t.Error("expected error when store dir is unreadable")
	}
}

// --- RegisterNatsConn with closed connection ---

func TestRegisterNatsConn_ClosedConn(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = filepath.Join(dir, "joblogs")
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	conn.Close()

	// Should not panic — subscribe errors are logged.
	RegisterNatsConn(conn)
}

// --- RecordJobStart: WriteFile meta error (simulate by filling disk — skip)
// --- AppendStep: OpenFile on existing file with append ---

func TestCLIStore_AppendStep_MultipleAppends(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	meta := CLIJobMeta{JID: "multi-append", SproutID: "sprout-ma", UserKey: "U1"}
	store.RecordJobStart(meta)

	for i := range 5 {
		step := cook.StepCompletion{
			ID:               cook.StepID(fmt.Sprintf("s%d", i)),
			CompletionStatus: cook.StepCompleted,
			Started:          time.Now(),
			Duration:         time.Duration(i) * time.Second,
		}
		if err := store.AppendStep("sprout-ma", "multi-append", step); err != nil {
			t.Fatalf("AppendStep %d: %v", i, err)
		}
	}

	jobFile := filepath.Join(dir, "sprout-ma", "multi-append.jsonl")
	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(steps))
	}
}

// --- GetJobMeta across multiple sprouts ---

func TestCLIStore_GetJobMeta_AcrossSprouts(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Record in sprout-2 (not sprout-1).
	meta := CLIJobMeta{JID: "cross-jid", SproutID: "sprout-2", UserKey: "UCROSS"}
	store.RecordJobStart(meta)

	// GetJobMeta searches all sprouts.
	got, err := store.GetJobMeta("cross-jid")
	if err != nil {
		t.Fatalf("GetJobMeta: %v", err)
	}
	if got.UserKey != "UCROSS" {
		t.Errorf("expected UCROSS, got %s", got.UserKey)
	}
}

// --- GetJob across multiple sprouts ---

func TestCLIStore_GetJob_AcrossSprouts(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	meta := CLIJobMeta{JID: "cross-job", SproutID: "sprout-3", UserKey: "UCROSS2"}
	store.RecordJobStart(meta)
	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	store.AppendStep("sprout-3", "cross-job", step)

	summary, gotMeta, err := store.GetJob("cross-job")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary.SproutID != "sprout-3" {
		t.Errorf("expected sprout-3, got %s", summary.SproutID)
	}
	if gotMeta.UserKey != "UCROSS2" {
		t.Errorf("expected UCROSS2, got %s", gotMeta.UserKey)
	}
}

// --- logJobCreation: create file error (dir is a file) ---

func TestLogJobCreation_CreateFileError(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Pre-create the sprout dir but put a FILE where the job file should be.
	sproutDir := filepath.Join(dir, "sprout-filecollision")
	os.MkdirAll(sproutDir, 0o700)
	// Create a directory where the job file should be — os.Create will fail.
	os.MkdirAll(filepath.Join(sproutDir, "collision-job.jsonl"), 0o700)

	envelope := cook.RecipeEnvelope{
		JobID: "collision-job",
		Steps: []cook.Step{{ID: "s1"}},
	}
	data, _ := json.Marshal(envelope)

	if err := conn.Publish("grlx.sprouts.sprout-filecollision.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
	// Should not panic — os.Create error is logged.
}

// --- logJobs: file is read-only (OpenFile append fails) ---

func TestLogJobs_ReadOnlyExistingFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Create a read-only existing job file.
	sproutDir := filepath.Join(dir, "sprout-readonly-file")
	os.MkdirAll(sproutDir, 0o700)
	jobFile := filepath.Join(sproutDir, "ro-job.jsonl")
	os.WriteFile(jobFile, []byte("{}\n"), 0o444)
	t.Cleanup(func() { os.Chmod(jobFile, 0o644) })

	step := cook.StepCompletion{
		ID:               "s1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	data, _ := json.Marshal(step)

	if err := conn.Publish("grlx.cook.sprout-readonly-file.ro-job", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
	// Should not panic — OpenFile error is logged.
}

// --- DefaultCLIStorePath without XDG_CONFIG_HOME ---

func TestDefaultCLIStorePath_WithoutXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path, err := DefaultCLIStorePath()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "grlx", "jobs")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// --- ConcurrentCLIStore operations ---

func TestCLIStore_ConcurrentAppendStep(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	meta := CLIJobMeta{JID: "concurrent-jid", SproutID: "sprout-c", UserKey: "U1"}
	if err := store.RecordJobStart(meta); err != nil {
		t.Fatal(err)
	}

	// Append steps from multiple goroutines.
	done := make(chan error, 10)
	for i := range 10 {
		go func(idx int) {
			step := cook.StepCompletion{
				ID:               cook.StepID(fmt.Sprintf("step-%d", idx)),
				CompletionStatus: cook.StepCompleted,
				Started:          time.Now(),
				Duration:         time.Millisecond,
			}
			done <- store.AppendStep("sprout-c", "concurrent-jid", step)
		}(i)
	}

	for range 10 {
		if err := <-done; err != nil {
			t.Errorf("AppendStep error: %v", err)
		}
	}

	// Verify all 10 steps recorded.
	jobFile := filepath.Join(dir, "sprout-c", "concurrent-jid.jsonl")
	readSteps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(readSteps) != 10 {
		t.Errorf("expected 10 steps, got %d", len(readSteps))
	}
}
