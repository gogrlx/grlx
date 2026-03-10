package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gogrlx/grlx/v2/internal/api"
	"github.com/gogrlx/grlx/v2/internal/auth"
	"github.com/gogrlx/grlx/v2/internal/config"
)

// CohortResolveResponse matches the farmer's response for POST /cohorts/resolve.
type CohortResolveResponse struct {
	Name    string   `json:"name"`
	Sprouts []string `json:"sprouts"`
}

// ResolveCohort asks the farmer to resolve a cohort name into sprout IDs.
func ResolveCohort(name string) ([]string, error) {
	client := APIClient
	ctx := context.Background()

	body, _ := json.Marshal(map[string]string{"name": name})
	url := config.FarmerURL + api.Routes["ResolveCohort"].Pattern
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
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

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact farmer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"]; ok {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("farmer returned status %d", resp.StatusCode)
	}

	var result CohortResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode cohort response: %w", err)
	}

	if len(result.Sprouts) == 0 {
		return nil, fmt.Errorf("cohort %q resolved to zero sprouts", name)
	}

	return result.Sprouts, nil
}
