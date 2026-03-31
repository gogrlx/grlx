package client

import (
	"encoding/json"
	"fmt"
)

// HealthResponse is the response from the health endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	UptimeMs  int64  `json:"uptime_ms"`
	NATSReady bool   `json:"nats_ready"`
}

// Health checks the farmer's health status over NATS.
func Health() (*HealthResponse, error) {
	resp, err := NatsRequest("health", nil)
	if err != nil {
		return nil, err
	}

	var result HealthResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("health: %w", err)
	}

	return &result, nil
}
