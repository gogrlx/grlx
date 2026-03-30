package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

// setupLocalJobsEnv creates a temp HOME with CLI job store data,
// sets HOME env var, and returns the original HOME for restoration.
func setupLocalJobsEnv(t *testing.T) (cleanup func()) {
	t.Helper()
	origHome := os.Getenv("HOME")
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	tmpHome := t.TempDir()

	// Set up store at $HOME/.config/grlx/jobs/
	storeDir := filepath.Join(tmpHome, ".config", "grlx", "jobs")
	sproutDir := filepath.Join(storeDir, "web-1")
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Write a .jsonl file for job "job-local-1".
	steps := []cook.StepCompletion{
		{ID: "start-job-local-1", CompletionStatus: cook.StepCompleted, Started: time.Date(2026, 3, 28, 3, 0, 0, 0, time.UTC)},
		{ID: "install-nginx", CompletionStatus: cook.StepCompleted, Started: time.Date(2026, 3, 28, 3, 0, 1, 0, time.UTC), Duration: 2 * time.Second, Changes: []string{"installed nginx 1.24"}},
		{ID: "completed-job-local-1", CompletionStatus: cook.StepCompleted},
	}
	var lines []byte
	for _, s := range steps {
		b, _ := json.Marshal(s)
		lines = append(lines, b...)
		lines = append(lines, '\n')
	}
	if err := os.WriteFile(filepath.Join(sproutDir, "job-local-1.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}

	// Write meta file.
	meta := jobs.CLIJobMeta{
		JID:       "job-local-1",
		SproutID:  "web-1",
		Recipe:    "base.packages",
		UserKey:   "TESTUSER123",
		CreatedAt: time.Date(2026, 3, 28, 3, 0, 0, 0, time.UTC),
	}
	metaBytes, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(sproutDir, "job-local-1.meta.json"), metaBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tmpHome)
	os.Unsetenv("XDG_CONFIG_HOME")

	return func() {
		os.Setenv("HOME", origHome)
		if origXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		}
	}
}

func TestListLocalJobs_Direct(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	// Save and restore globals.
	oldLimit := jobsLimit
	oldUser := jobsUser
	defer func() { jobsLimit = oldLimit; jobsUser = oldUser }()

	jobsLimit = 50
	jobsUser = ""

	summaries, err := listLocalJobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job, got %d", len(summaries))
	}
	if summaries[0].JID != "job-local-1" {
		t.Errorf("expected job-local-1, got %s", summaries[0].JID)
	}
}

func TestListLocalJobs_WithSproutFilter(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	oldLimit := jobsLimit
	oldUser := jobsUser
	defer func() { jobsLimit = oldLimit; jobsUser = oldUser }()

	jobsLimit = 50
	jobsUser = ""

	summaries, err := listLocalJobs([]string{"web-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job for web-1, got %d", len(summaries))
	}

	// Non-existent sprout.
	summaries, err = listLocalJobs([]string{"nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 jobs for nonexistent sprout, got %d", len(summaries))
	}
}

func TestListLocalJobs_WithUserFilter(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	oldLimit := jobsLimit
	oldUser := jobsUser
	defer func() { jobsLimit = oldLimit; jobsUser = oldUser }()

	jobsLimit = 50

	// Filter by matching user.
	jobsUser = "TESTUSER123"
	summaries, err := listLocalJobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job for TESTUSER123, got %d", len(summaries))
	}

	// Filter by non-matching user.
	jobsUser = "OTHER_USER"
	summaries, err = listLocalJobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 jobs for OTHER_USER, got %d", len(summaries))
	}
}

func TestListLocalJobs_WithLimit(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	oldLimit := jobsLimit
	oldUser := jobsUser
	defer func() { jobsLimit = oldLimit; jobsUser = oldUser }()

	jobsUser = ""
	jobsLimit = 0 // 0 means no limit in the store

	summaries, err := listLocalJobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job, got %d", len(summaries))
	}
}

func TestShowLocalJob_TextMode(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	// Save and restore outputMode.
	oldMode := outputMode
	defer func() { outputMode = oldMode }()

	outputMode = "text"

	out := captureStdout(t, func() {
		showLocalJob("job-local-1")
	})

	if !strings.Contains(out, "job-local-1") {
		t.Error("expected JID in text output")
	}
	if !strings.Contains(out, "web-1") {
		t.Error("expected sprout ID in text output")
	}
	if !strings.Contains(out, "TESTUSER123") {
		t.Error("expected user key in text output")
	}
	if !strings.Contains(out, "base.packages") {
		t.Error("expected recipe in text output")
	}
}

func TestShowLocalJob_JSONMode(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	oldMode := outputMode
	defer func() { outputMode = oldMode }()

	outputMode = "json"

	out := captureStdout(t, func() {
		showLocalJob("job-local-1")
	})

	if !strings.Contains(out, "job-local-1") {
		t.Error("expected JID in JSON output")
	}
	if !strings.Contains(out, "TESTUSER123") {
		t.Error("expected user key in JSON output")
	}
	if !strings.Contains(out, "base.packages") {
		t.Error("expected recipe in JSON output")
	}

	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestShowLocalJob_DefaultMode(t *testing.T) {
	cleanup := setupLocalJobsEnv(t)
	defer cleanup()

	oldMode := outputMode
	defer func() { outputMode = oldMode }()

	outputMode = ""

	out := captureStdout(t, func() {
		showLocalJob("job-local-1")
	})

	// Default mode should behave like text.
	if !strings.Contains(out, "job-local-1") {
		t.Error("expected JID in default output")
	}
	if !strings.Contains(out, "TESTUSER123") {
		t.Error("expected user key in default output")
	}
}

// --- CLIStore direct tests for coverage of the package under test ---

func TestCLIStoreDirect_NoFilter(t *testing.T) {
	dir := t.TempDir()
	sproutDir := filepath.Join(dir, "app-1")
	if err := os.MkdirAll(sproutDir, 0o700); err != nil {
		t.Fatal(err)
	}

	steps := []cook.StepCompletion{
		{ID: "start-j1", CompletionStatus: cook.StepCompleted, Started: time.Date(2026, 3, 28, 3, 0, 0, 0, time.UTC)},
		{ID: "real-step", CompletionStatus: cook.StepCompleted},
		{ID: "completed-j1", CompletionStatus: cook.StepCompleted},
	}
	var lines []byte
	for _, s := range steps {
		b, _ := json.Marshal(s)
		lines = append(lines, b...)
		lines = append(lines, '\n')
	}
	if err := os.WriteFile(filepath.Join(sproutDir, "j1.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := jobs.NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	summaries, err := store.ListJobs(50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job, got %d", len(summaries))
	}
}

func TestCLIStoreDirect_GetJobNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = store.GetJob("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}
