package natsapi

import (
	"encoding/json"
	"time"
)

var farmerStartTime time.Time

func init() {
	farmerStartTime = time.Now()
}

// HealthResponse is the response for the health endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	UptimeMs  int64  `json:"uptime_ms"`
	NATSReady bool   `json:"nats_ready"`
}

func handleHealth(_ json.RawMessage) (any, error) {
	uptime := time.Since(farmerStartTime)
	natsReady := natsConn != nil && natsConn.IsConnected()

	status := "ok"
	if !natsReady {
		status = "degraded"
	}

	return HealthResponse{
		Status:    status,
		Uptime:    uptime.Round(time.Second).String(),
		UptimeMs:  uptime.Milliseconds(),
		NATSReady: natsReady,
	}, nil
}
