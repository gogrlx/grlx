package natsapi

import (
	"testing"
	"time"
)

func TestHandleHealth(t *testing.T) {
	// Save and restore farmerStartTime.
	origStart := farmerStartTime
	farmerStartTime = time.Now().Add(-2 * time.Hour)
	defer func() { farmerStartTime = origStart }()

	// No NATS connection — should report degraded.
	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	result, err := handleHealth(nil)
	if err != nil {
		t.Fatalf("handleHealth: unexpected error: %v", err)
	}

	resp, ok := result.(HealthResponse)
	if !ok {
		t.Fatalf("result type = %T, want HealthResponse", result)
	}

	if resp.Status != "degraded" {
		t.Errorf("Status = %q, want %q", resp.Status, "degraded")
	}
	if resp.NATSReady {
		t.Error("NATSReady = true, want false (nil connection)")
	}
	if resp.UptimeMs < 7200000 {
		t.Errorf("UptimeMs = %d, expected at least 7200000 (2h)", resp.UptimeMs)
	}
	if resp.Uptime == "" {
		t.Error("Uptime should not be empty")
	}
}

func TestHandleHealthOK(t *testing.T) {
	origStart := farmerStartTime
	farmerStartTime = time.Now().Add(-10 * time.Minute)
	defer func() { farmerStartTime = origStart }()

	nc, cleanup := startEmbeddedNATS(t)
	defer cleanup()

	old := natsConn
	natsConn = nc
	defer func() { natsConn = old }()

	result, err := handleHealth(nil)
	if err != nil {
		t.Fatalf("handleHealth: unexpected error: %v", err)
	}

	resp := result.(HealthResponse)
	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
	if !resp.NATSReady {
		t.Error("NATSReady = false, want true")
	}
}

func TestHandleHealthUptime(t *testing.T) {
	origStart := farmerStartTime
	farmerStartTime = time.Now().Add(-30 * time.Second)
	defer func() { farmerStartTime = origStart }()

	old := natsConn
	natsConn = nil
	defer func() { natsConn = old }()

	result, err := handleHealth(nil)
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}

	resp := result.(HealthResponse)
	if resp.UptimeMs < 30000 {
		t.Errorf("UptimeMs = %d, expected at least 30000", resp.UptimeMs)
	}
}
