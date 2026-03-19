package client

import (
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/jobs"
)

func TestListJobs_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := []jobs.JobSummary{
		{JID: "jid-001", SproutID: "web-01", Status: jobs.JobSucceeded, StartedAt: time.Now().UTC(), Total: 5, Succeeded: 5},
		{JID: "jid-002", SproutID: "db-01", Status: jobs.JobRunning, StartedAt: time.Now().UTC(), Total: 3, Succeeded: 1},
	}
	mockHandler(t, NatsConn, "grlx.api.jobs.list", want)

	got, err := ListJobs(10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(got))
	}
	if got[0].JID != "jid-001" {
		t.Fatalf("expected jid-001, got %q", got[0].JID)
	}
	if got[1].Status != jobs.JobRunning {
		t.Fatalf("expected JobRunning, got %d", got[1].Status)
	}
}

func TestListJobs_Empty(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockHandler(t, NatsConn, "grlx.api.jobs.list", []jobs.JobSummary{})

	got, err := ListJobs(10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(got))
	}
}

func TestListJobs_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.jobs.list", "database error")

	_, err := ListJobs(10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetJob_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := jobs.JobSummary{
		JID:       "jid-abc",
		SproutID:  "web-01",
		Status:    jobs.JobSucceeded,
		StartedAt: time.Now().UTC(),
		Total:     3,
		Succeeded: 3,
	}
	mockHandler(t, NatsConn, "grlx.api.jobs.get", want)

	got, err := GetJob("jid-abc")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.JID != "jid-abc" {
		t.Fatalf("expected jid-abc, got %q", got.JID)
	}
	if got.Succeeded != 3 {
		t.Fatalf("expected 3 succeeded, got %d", got.Succeeded)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.jobs.get", "job not found")

	_, err := GetJob("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListJobsForSprout_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := []jobs.JobSummary{
		{JID: "jid-100", SproutID: "db-01", Status: jobs.JobSucceeded, Total: 2, Succeeded: 2},
	}
	mockHandler(t, NatsConn, "grlx.api.jobs.forsprout", want)

	got, err := ListJobsForSprout("db-01")
	if err != nil {
		t.Fatalf("ListJobsForSprout: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 job, got %d", len(got))
	}
	if got[0].SproutID != "db-01" {
		t.Fatalf("expected sprout db-01, got %q", got[0].SproutID)
	}
}

func TestCancelJob_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockHandler(t, NatsConn, "grlx.api.jobs.cancel", map[string]bool{"ok": true})

	err := CancelJob("jid-cancel-me")
	if err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
}

func TestCancelJob_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.jobs.cancel", "job already completed")

	err := CancelJob("jid-done")
	if err == nil {
		t.Fatal("expected error")
	}
}
