package client

import (
	"testing"
)

func TestHealth_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := HealthResponse{
		Status:    "ok",
		Uptime:    "1h30m0s",
		UptimeMs:  5400000,
		NATSReady: true,
	}
	mockHandler(t, NatsConn, "grlx.api.health", want)

	got, err := Health()
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.Status != "ok" {
		t.Errorf("Status = %q, want %q", got.Status, "ok")
	}
	if got.Uptime != "1h30m0s" {
		t.Errorf("Uptime = %q, want %q", got.Uptime, "1h30m0s")
	}
	if got.UptimeMs != 5400000 {
		t.Errorf("UptimeMs = %d, want %d", got.UptimeMs, 5400000)
	}
	if !got.NATSReady {
		t.Error("NATSReady = false, want true")
	}
}

func TestHealth_Degraded(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := HealthResponse{
		Status:    "degraded",
		Uptime:    "5m0s",
		UptimeMs:  300000,
		NATSReady: false,
	}
	mockHandler(t, NatsConn, "grlx.api.health", want)

	got, err := Health()
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.Status != "degraded" {
		t.Errorf("Status = %q, want %q", got.Status, "degraded")
	}
	if got.NATSReady {
		t.Error("NATSReady = true, want false")
	}
}

func TestHealth_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.health", "farmer unreachable")

	_, err := Health()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHealth_BadJSON(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockBadJSONHandler(t, NatsConn, "grlx.api.health")

	_, err := Health()
	if err == nil {
		t.Fatal("expected unmarshal error for bad JSON")
	}
}

func TestHealth_NoConnection(t *testing.T) {
	old := NatsConn
	NatsConn = nil
	defer func() { NatsConn = old }()

	_, err := Health()
	if err == nil {
		t.Fatal("expected error for nil NatsConn")
	}
}
