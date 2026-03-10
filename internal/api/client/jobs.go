package client

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/jobs"
)

// ListJobs retrieves all recent jobs from the farmer, up to limit.
func ListJobs(limit int) ([]jobs.JobSummary, error) {
	params := map[string]int{"limit": limit}
	resp, err := NatsRequest("jobs.list", params)
	if err != nil {
		return nil, err
	}
	var summaries []jobs.JobSummary
	if err := json.Unmarshal(resp, &summaries); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	return summaries, nil
}

// GetJob retrieves a specific job by JID from the farmer.
func GetJob(jid string) (*jobs.JobSummary, error) {
	params := map[string]string{"jid": jid}
	resp, err := NatsRequest("jobs.get", params)
	if err != nil {
		return nil, err
	}
	var summary jobs.JobSummary
	if err := json.Unmarshal(resp, &summary); err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &summary, nil
}

// ListJobsForSprout retrieves all jobs for a specific sprout.
func ListJobsForSprout(sproutID string) ([]jobs.JobSummary, error) {
	params := map[string]string{"sprout_id": sproutID}
	resp, err := NatsRequest("jobs.forsprout", params)
	if err != nil {
		return nil, err
	}
	var summaries []jobs.JobSummary
	if err := json.Unmarshal(resp, &summaries); err != nil {
		return nil, fmt.Errorf("list jobs for sprout: %w", err)
	}
	return summaries, nil
}

// CancelJob sends a cancel request for a job to the farmer.
func CancelJob(jid string) error {
	params := map[string]string{"jid": jid}
	_, err := NatsRequest("jobs.cancel", params)
	return err
}
