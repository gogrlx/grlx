package client

import (
	"encoding/json"
	"fmt"
	"time"
)

// CohortResolveResponse matches the farmer's response for cohort resolution.
type CohortResolveResponse struct {
	Name    string   `json:"name"`
	Sprouts []string `json:"sprouts"`
}

// CohortRefreshResult describes one refreshed cohort.
type CohortRefreshResult struct {
	Name          string    `json:"name"`
	Members       []string  `json:"members"`
	LastRefreshed time.Time `json:"lastRefreshed"`
}

// CohortRefreshResponse is the response for a cohort refresh request.
type CohortRefreshResponse struct {
	Refreshed []CohortRefreshResult `json:"refreshed"`
}

// CohortDetail is the full definition of a cohort returned by cohorts.get.
type CohortDetail struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Members []string `json:"members,omitempty"`
	Match   *struct {
		PropName  string `json:"propName"`
		PropValue string `json:"propValue"`
	} `json:"match,omitempty"`
	Compound *struct {
		Operator string   `json:"operator"`
		Operands []string `json:"operands"`
	} `json:"compound,omitempty"`
	Resolved []string `json:"resolved"`
	Count    int      `json:"count"`
}

// GetCohort fetches the full definition of a named cohort from the farmer.
func GetCohort(name string) (*CohortDetail, error) {
	params := map[string]string{"name": name}
	resp, err := NatsRequest("cohorts.get", params)
	if err != nil {
		return nil, err
	}

	var detail CohortDetail
	if err := json.Unmarshal(resp, &detail); err != nil {
		return nil, fmt.Errorf("get cohort: %w", err)
	}

	return &detail, nil
}

// ResolveCohort asks the farmer to resolve a cohort name into sprout IDs.
func ResolveCohort(name string) ([]string, error) {
	params := map[string]string{"name": name}
	resp, err := NatsRequest("cohorts.resolve", params)
	if err != nil {
		return nil, err
	}

	var result CohortResolveResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("resolve cohort: %w", err)
	}

	if len(result.Sprouts) == 0 {
		return nil, fmt.Errorf("cohort %q resolved to zero sprouts", name)
	}

	return result.Sprouts, nil
}

// RefreshCohort refreshes a single cohort's membership cache.
func RefreshCohort(name string) (*CohortRefreshResponse, error) {
	params := map[string]string{"name": name}
	resp, err := NatsRequest("cohorts.refresh", params)
	if err != nil {
		return nil, err
	}

	var result CohortRefreshResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("refresh cohort: %w", err)
	}
	return &result, nil
}

// RefreshAllCohorts refreshes membership cache for all cohorts.
func RefreshAllCohorts() (*CohortRefreshResponse, error) {
	resp, err := NatsRequest("cohorts.refresh", nil)
	if err != nil {
		return nil, err
	}

	var result CohortRefreshResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("refresh cohorts: %w", err)
	}
	return &result, nil
}
