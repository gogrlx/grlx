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

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

// serveWithPathValues routes a request through a real ServeMux so that
// r.PathValue() is populated by the stdlib router.
func serveWithPathValues(pattern string, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, handler)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

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
	w := serveWithPathValues("GET /jobs/{jid}", GetJob, req)

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
	w := serveWithPathValues("GET /jobs/{jid}", GetJob, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestListJobsForSprout(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/sprout/sprout-alpha", nil)
	w := serveWithPathValues("GET /jobs/sprout/{sproutID}", ListJobsForSprout, req)

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
	w := serveWithPathValues("GET /jobs/sprout/{sproutID}", ListJobsForSprout, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	if w.Body.String() != "[]" {
		t.Fatalf("expected empty array, got %s", w.Body.String())
	}
}

func TestCancelJobNoNATS(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodDelete, "/jobs/jid-002", nil)

	// Ensure conn is nil for this test
	oldConn := conn
	conn = nil
	defer func() { conn = oldConn }()

	w := serveWithPathValues("DELETE /jobs/{jid}", CancelJob, req)

	// Without NATS, cancel should return 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 without NATS, got %d", w.Code)
	}
}

func TestCancelJobNotFound(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodDelete, "/jobs/nonexistent", nil)
	w := serveWithPathValues("DELETE /jobs/{jid}", CancelJob, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestCancelJobAlreadyCompleted(t *testing.T) {
	setupTestJobStore(t)

	// jid-001 is a completed job (all steps succeeded)
	req := httptest.NewRequest(http.MethodDelete, "/jobs/jid-001", nil)
	w := serveWithPathValues("DELETE /jobs/{jid}", CancelJob, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestListJobsNegativeLimit(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=-1", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for negative limit, got %d", w.Code)
	}
}

func TestListJobsZeroLimit(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=0", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for zero limit, got %d", w.Code)
	}
}

func TestGetJob_MissingJID(t *testing.T) {
	setupTestJobStore(t)

	// Call directly without mux so PathValue("jid") returns "".
	req := httptest.NewRequest(http.MethodGet, "/jobs/", nil)
	w := httptest.NewRecorder()

	GetJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing jid, got %d", w.Code)
	}
}

func TestListJobsForSprout_MissingSproutID(t *testing.T) {
	setupTestJobStore(t)

	// Call directly without mux so PathValue("sproutID") returns "".
	req := httptest.NewRequest(http.MethodGet, "/jobs/sprout/", nil)
	w := httptest.NewRecorder()

	ListJobsForSprout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing sproutID, got %d", w.Code)
	}
}

func TestCancelJob_MissingJID(t *testing.T) {
	setupTestJobStore(t)

	// Call directly without mux so PathValue("jid") returns "".
	req := httptest.NewRequest(http.MethodDelete, "/jobs/", nil)
	w := httptest.NewRecorder()

	CancelJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing jid, got %d", w.Code)
	}
}

func TestCancelJob_FailedJob(t *testing.T) {
	setupTestJobStore(t)

	// jid-003 is a failed job
	req := httptest.NewRequest(http.MethodDelete, "/jobs/jid-003", nil)
	w := serveWithPathValues("DELETE /jobs/{jid}", CancelJob, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409 for failed job, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "job cannot be cancelled" {
		t.Errorf("expected 'job cannot be cancelled', got %q", resp["error"])
	}
}

func TestListJobsLargeLimit(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs?limit=1000", nil)
	w := httptest.NewRecorder()

	ListJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summaries []jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Should return all 3 jobs
	if len(summaries) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(summaries))
	}
}

func TestGetJob_VerifyFields(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/jid-002", nil)
	w := serveWithPathValues("GET /jobs/{jid}", GetJob, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summary jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.JID != "jid-002" {
		t.Errorf("expected JID jid-002, got %s", summary.JID)
	}
	if summary.SproutID != "sprout-alpha" {
		t.Errorf("expected sprout sprout-alpha, got %s", summary.SproutID)
	}
}

func TestListJobsForSprout_Beta(t *testing.T) {
	setupTestJobStore(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/sprout/sprout-beta", nil)
	w := serveWithPathValues("GET /jobs/sprout/{sproutID}", ListJobsForSprout, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var summaries []jobs.JobSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 job for sprout-beta, got %d", len(summaries))
	}
	if summaries[0].JID != "jid-003" {
		t.Errorf("expected jid-003, got %s", summaries[0].JID)
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
