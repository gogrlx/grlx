package client

import (
	"encoding/json"
	"fmt"

	"github.com/gogrlx/grlx/v2/internal/audit"
)

// ListAuditDates returns a summary of all available audit log dates.
func ListAuditDates() ([]audit.DateSummary, error) {
	resp, err := NatsRequest("audit.dates", nil)
	if err != nil {
		return nil, err
	}
	var dates []audit.DateSummary
	if err := json.Unmarshal(resp, &dates); err != nil {
		return nil, fmt.Errorf("audit dates: %w", err)
	}
	return dates, nil
}

// QueryAudit queries audit log entries with the given parameters.
func QueryAudit(params audit.QueryParams) (audit.QueryResult, error) {
	var result audit.QueryResult
	resp, err := NatsRequest("audit.query", params)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return result, fmt.Errorf("audit query: %w", err)
	}
	return result, nil
}
