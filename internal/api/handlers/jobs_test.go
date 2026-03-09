package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

// setupTestJobStore creates a temp directory with test job data and replaces
// the package-level jobStore.
func setupTestJobStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create test job files
	writeJobFile(t, dir, "sprout-alpha", "jid-001", []cook.StepCompletion{
		makeStep("step1", cook.StepCompleted, time.Now().Add(-2*time.Hour), time.Minute),
		makeStep("step2", cook.StepCompleted, time.Now().Add(-2*time.Hour+time.Minute), time.Minute),
	})
	writeJobFile(t, dir, "sprout-alpha", "jid-002", []cook.StepCompletion{
		makeStep("step1", cook.StepInProgress, time.Now().Add(-5*time.Minute), 0),
	})
	writeJobFile(t, dir, "sprout-beta", "jid-003", []cook.StepCompletion{
		makeStep("step1", cook.StepFailed, time.Now().Add(-time.Hour), 30*time.Second),
	})

	jobStore = jobs.NewStoreWithDir(dir)
	return dir
}

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

func TestListJobs(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summaries []jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(summaries))
	}
}

func TestListJobsWithLimit(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=2", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summaries []jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(summaries))
	}
}

func TestListJobsInvalidLimit(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=abc", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetJob(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/jid-001", nil)
	req = mux.SetURLVars(req, map[string]string{"jid": "jid-001"})
	w := httptest.NewRecorder()

	GetJob(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summary jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if summary.JID != "jid-001" {
		t.Fatalf("expected JID jid-001, got %s", summary.JID)
	}
	if summary.SproutID != "sprout-alpha" {
		t.Fatalf("expected sprout sprout-alpha, got %s", summary.SproutID)
	}
	if summary.Total != 2 {
		t.Fatalf("expected 2 steps, got %d", summary.Total)
	}
}

func TestGetJobNotFound(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"jid": "nonexistent"})
	w := httptest.NewRecorder()

	GetJob(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestGetJobMissingJID(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/", nil)
	req = mux.SetURLVars(req, map[string]string{})
	w := httptest.NewRecorder()

	GetJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestListJobsForSprout(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/sprout/sprout-alpha", nil)
	req = mux.SetURLVars(req, map[string]string{"sproutID": "sprout-alpha"})
	w := httptest.NewRecorder()

	ListJobsForSprout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summaries []jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("expected 2 jobs for sprout-alpha, got %d", len(summaries))
	}
}

func TestListJobsForSproutNoJobs(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/sprout/sprout-unknown", nil)
	req = mux.SetURLVars(req, map[string]string{"sproutID": "sprout-unknown"})
	w := httptest.NewRecorder()

	ListJobsForSprout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	if w.Body.String() != "[]" {
		t.Fatalf("expected empty array, got %s", w.Body.String())
	}
}

func TestCancelJobNoNATS(t *testing.T) {
	setupTestJobStore(t)
	// conn is nil by default in tests (no NATS)

	req := httptest.NewRequest(http.MethodDelete, "/jobs/jid-002", nil)
	req = mux.SetURLVars(req, map[string]string{"jid": "jid-002"})
	w := httptest.NewRecorder()

	// Ensure conn is nil for this test
	oldConn := conn
	conn = nil
	defer func() { conn = oldConn }()

	CancelJob(w, req)

	// Without NATS, cancel should return 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 without NATS, got %d", w.Code)
	}
}

func TestCancelJobNotFound(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodDelete, "/jobs/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"jid": "nonexistent"})
	w := httptest.NewRecorder()

	CancelJob(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestCancelJobAlreadyCompleted(t *testing.T) {
	setupTestJobStore(t)

	// jid-001 is a completed job (all steps succeeded)
	req := httptest.NewRequest(http.MethodDelete, "/jobs/jid-001", nil)
	req = mux.SetURLVars(req, map[string]string{"jid": "jid-001"})
	w := httptest.NewRecorder()

	CancelJob(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestListJobsEmptyStore(t *testing.T) {
	dir := t.TempDir()
	jobStore = jobs.NewStoreWithDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summaries []jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(summaries) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(summaries))
	}
}
