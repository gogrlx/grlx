package client

import (
	"encoding/json"
	"fmt"
)

// CohortResolveResponse matches the farmer's response for cohort resolution.
type CohortResolveResponse struct {
	Name    string   `json:"name"`
	Sprouts []string `json:"sprouts"`
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
