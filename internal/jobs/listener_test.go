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

func TestRegisterNatsConn(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = filepath.Join(dir, "joblogs")
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)

	// Should create the jobs directory.
	RegisterNatsConn(conn)

	if _, err := os.Stat(config.JobLogDir); err != nil {
		t.Errorf("expected job log dir to be created: %v", err)
	}
}

func TestRegisterNatsConn_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)

	// Should not error when dir already exists.
	RegisterNatsConn(conn)
}

func TestLogJobs_StepCompletion(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         3 * time.Second,
	}
	data, _ := json.Marshal(step)

	if err := conn.Publish("grlx.cook.sprout-log-test.job-log-1", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	// Verify the job file was created.
	jobFile := filepath.Join(dir, "sprout-log-test", "job-log-1.jsonl")
	if _, err := os.Stat(jobFile); err != nil {
		t.Fatalf("expected job file to exist: %v", err)
	}

	// Read and verify contents.
	content, err := os.ReadFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Fatal("expected non-empty job file")
	}
}

func TestLogJobs_AppendToExisting(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Publish two steps.
	for i := range 2 {
		step := cook.StepCompletion{
			ID:               cook.StepID(fmt.Sprintf("step-%d", i)),
			CompletionStatus: cook.StepCompleted,
			Started:          time.Now(),
			Duration:         time.Second,
		}
		data, _ := json.Marshal(step)
		if err := conn.Publish("grlx.cook.sprout-append.job-append-1", data); err != nil {
			t.Fatal(err)
		}
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	// Verify both steps are in the file.
	jobFile := filepath.Join(dir, "sprout-append", "job-append-1.jsonl")
	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
}

func TestLogJobs_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Publish invalid JSON — should not panic.
	if err := conn.Publish("grlx.cook.sprout-bad.job-bad", []byte("invalid json")); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Job file should not exist (invalid data was rejected).
	jobFile := filepath.Join(dir, "sprout-bad", "job-bad.jsonl")
	if _, err := os.Stat(jobFile); err == nil {
		t.Error("expected job file to NOT exist for invalid JSON")
	}
}

func TestLogJobCreation(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	envelope := cook.RecipeEnvelope{
		JobID:     "creation-job-1",
		InvokedBy: "UPUBKEY_CREATOR",
		Steps: []cook.Step{
			{ID: "step-a"},
			{ID: "step-b"},
		},
	}
	data, _ := json.Marshal(envelope)

	if err := conn.Publish("grlx.sprouts.sprout-create.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(300 * time.Millisecond)

	// Verify job file was created with placeholder steps.
	jobFile := filepath.Join(dir, "sprout-create", "creation-job-1.jsonl")
	if _, err := os.Stat(jobFile); err != nil {
		t.Fatalf("expected job file to exist: %v", err)
	}

	steps, err := readJobFile(jobFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 placeholder steps, got %d", len(steps))
	}
	for _, step := range steps {
		if step.CompletionStatus != cook.StepNotStarted {
			t.Errorf("expected StepNotStarted, got %d", step.CompletionStatus)
		}
	}

	// Verify meta file was created.
	metaFile := filepath.Join(dir, "sprout-create", "creation-job-1.meta.json")
	if _, err := os.Stat(metaFile); err != nil {
		t.Fatalf("expected meta file to exist: %v", err)
	}

	invokedBy := readJobMeta(filepath.Join(dir, "sprout-create"), "creation-job-1")
	if invokedBy != "UPUBKEY_CREATOR" {
		t.Errorf("expected UPUBKEY_CREATOR, got %s", invokedBy)
	}
}

func TestLogJobCreation_EmptyJobID(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Envelope with empty JobID should be ignored.
	envelope := cook.RecipeEnvelope{
		JobID: "",
		Steps: []cook.Step{{ID: "step-a"}},
	}
	data, _ := json.Marshal(envelope)

	if err := conn.Publish("grlx.sprouts.sprout-empty.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Sprout dir should not exist.
	sproutDir := filepath.Join(dir, "sprout-empty")
	entries, _ := os.ReadDir(sproutDir)
	if len(entries) > 0 {
		t.Error("expected no files for empty job ID envelope")
	}
}

func TestLogJobCreation_NoInvokedBy(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	envelope := cook.RecipeEnvelope{
		JobID: "no-invoker-job",
		Steps: []cook.Step{{ID: "step-a"}},
	}
	data, _ := json.Marshal(envelope)

	if err := conn.Publish("grlx.sprouts.sprout-noinv.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Job file should exist, but no meta file.
	jobFile := filepath.Join(dir, "sprout-noinv", "no-invoker-job.jsonl")
	if _, err := os.Stat(jobFile); err != nil {
		t.Fatalf("expected job file: %v", err)
	}
	metaFile := filepath.Join(dir, "sprout-noinv", "no-invoker-job.meta.json")
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		t.Error("expected no meta file when InvokedBy is empty")
	}
}

func TestLogJobs_JobFileIsDirectory(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	// Pre-create a directory where the job file should be — triggers the "is a directory" path.
	sproutDir := filepath.Join(dir, "sprout-dirjob")
	os.MkdirAll(filepath.Join(sproutDir, "dir-as-job.jsonl"), 0o700)

	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	data, _ := json.Marshal(step)

	// Should handle gracefully — the path is a directory.
	if err := conn.Publish("grlx.cook.sprout-dirjob.dir-as-job", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)
}

func TestLogJobCreation_DuplicateJobID(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	envelope := cook.RecipeEnvelope{
		JobID: "dup-job",
		Steps: []cook.Step{{ID: "step-a"}},
	}
	data, _ := json.Marshal(envelope)

	// First creation.
	if err := conn.Publish("grlx.sprouts.sprout-dup.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Get content after first creation.
	jobFile := filepath.Join(dir, "sprout-dup", "dup-job.jsonl")
	originalContent, _ := os.ReadFile(jobFile)

	// Second creation with same JID — should be a no-op since file exists.
	if err := conn.Publish("grlx.sprouts.sprout-dup.cook", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Content should be unchanged.
	afterContent, _ := os.ReadFile(jobFile)
	if string(originalContent) != string(afterContent) {
		t.Error("expected duplicate job creation to be a no-op")
	}
}

func TestLogJobCreation_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	origJobLogDir := config.JobLogDir
	config.JobLogDir = dir
	t.Cleanup(func() { config.JobLogDir = origJobLogDir })

	_, conn := startTestNATSServer(t)
	RegisterNatsConn(conn)

	if err := conn.Publish("grlx.sprouts.sprout-badjson.cook", []byte("not json")); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Nothing should be created.
	sproutDir := filepath.Join(dir, "sprout-badjson")
	if _, err := os.Stat(sproutDir); err == nil {
		entries, _ := os.ReadDir(sproutDir)
		if len(entries) > 0 {
			t.Error("expected no files for invalid JSON")
		}
	}
}
