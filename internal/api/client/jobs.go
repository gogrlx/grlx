package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/jobs"
)

// ListJobs retrieves all recent jobs from the farmer, up to limit.
func ListJobs(limit int) ([]jobs.JobSummary, error) {
	farmerURL := config.FarmerURL
	url := farmerURL + api.Routes["ListJobs"].Pattern
	if limit > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, limit)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	newToken, err := auth.NewToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", newToken)

	resp, err := APIClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from farmer", resp.StatusCode)
	}

	var summaries []jobs.JobSummary
	if err := json.NewDecoder(resp.Body).Decode(&summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

// GetJob retrieves a specific job by JID from the farmer.
func GetJob(jid string) (*jobs.JobSummary, error) {
	farmerURL := config.FarmerURL
	// Build URL manually since the route pattern has {jid} placeholder.
	url := fmt.Sprintf("%s/jobs/%s", farmerURL, jid)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	newToken, err := auth.NewToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", newToken)

	resp, err := APIClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, jobs.ErrJobNotFound
	default:
		return nil, fmt.Errorf("unexpected status %d from farmer", resp.StatusCode)
	}

	var summary jobs.JobSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// ListJobsForSprout retrieves all jobs for a specific sprout.
func ListJobsForSprout(sproutID string) ([]jobs.JobSummary, error) {
	farmerURL := config.FarmerURL
	url := fmt.Sprintf("%s/jobs/sprout/%s", farmerURL, sproutID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	newToken, err := auth.NewToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", newToken)

	resp, err := APIClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from farmer", resp.StatusCode)
	}

	var summaries []jobs.JobSummary
	if err := json.NewDecoder(resp.Body).Decode(&summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

// CancelJob sends a cancel request for a job to the farmer.
func CancelJob(jid string) error {
	farmerURL := config.FarmerURL
	url := fmt.Sprintf("%s/jobs/%s", farmerURL, jid)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	newToken, err := auth.NewToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", newToken)

	resp, err := APIClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusNotFound:
		return jobs.ErrJobNotFound
	case http.StatusConflict:
		return fmt.Errorf("job cannot be cancelled (already completed)")
	default:
		return fmt.Errorf("unexpected status %d from farmer", resp.StatusCode)
	}
}
