package natsapi

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/jobs"
)

var jobStore *jobs.Store

func init() {
	jobStore = jobs.NewStore()
}

// JobsListParams holds optional parameters for listing jobs.
type JobsListParams struct {
	Limit int `json:"limit,omitempty"`
}

// JobsGetParams identifies a job by JID.
type JobsGetParams struct {
	JID string `json:"jid"`
}

// JobsForSproutParams identifies a sprout for job listing.
type JobsForSproutParams struct {
	SproutID string `json:"sprout_id"`
}

func handleJobsList(params json.RawMessage) (any, error) {
	var p JobsListParams
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	summaries, err := jobStore.ListAllJobs(limit)
	if err != nil {
		return nil, err
	}
	if summaries == nil {
		summaries = []jobs.JobSummary{}
	}
	return summaries, nil
}

func handleJobsGet(params json.RawMessage) (any, error) {
	var p JobsGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.JID == "" {
		return nil, fmt.Errorf("jid is required")
	}
	summary, err := jobStore.FindJob(p.JID)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func handleJobsCancel(params json.RawMessage) (any, error) {
	var p JobsGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.JID == "" {
		return nil, fmt.Errorf("jid is required")
	}

	summary, err := jobStore.FindJob(p.JID)
	if err != nil {
		return nil, err
	}

	if summary.Status != jobs.JobRunning && summary.Status != jobs.JobPending {
		return nil, fmt.Errorf("job cannot be cancelled: status is %s", summary.Status)
	}

	subject := fmt.Sprintf("grlx.sprouts.%s.cancel", summary.SproutID)
	cancelMsg, _ := json.Marshal(map[string]string{"jid": p.JID})

	if natsConn == nil {
		return nil, fmt.Errorf("NATS connection not available")
	}

	if err := natsConn.Publish(subject, cancelMsg); err != nil {
		return nil, fmt.Errorf("failed to publish cancel: %w", err)
	}

	return map[string]string{
		"jid":     p.JID,
		"sprout":  summary.SproutID,
		"message": "cancel request published",
	}, nil
}

func handleJobsListForSprout(params json.RawMessage) (any, error) {
	var p JobsForSproutParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SproutID == "" {
		return nil, fmt.Errorf("sprout_id is required")
	}

	summaries, err := jobStore.ListJobsForSprout(p.SproutID)
	if err != nil {
		return nil, err
	}
	if summaries == nil {
		summaries = []jobs.JobSummary{}
	}
	return summaries, nil
}
