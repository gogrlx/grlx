package facts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/props"
)

func startTestNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start test NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to become ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect to test NATS: %v", err)
	}
	return nc, func() {
		nc.Close()
		ns.Shutdown()
	}
}

func TestRegisterFarmerListener_ValidFacts(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	RegisterFarmerListener(nc)
	nc.Flush()

	sf := SystemFacts{
		OS:          "linux",
		Arch:        "amd64",
		Hostname:    "listener-test-host",
		GoVersion:   "go1.22.0",
		NumCPU:      4,
		IPAddresses: []string{"10.0.0.1"},
		KernelArch:  "amd64",
		SproutID:    "sprout-listener-test",
	}
	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if err := nc.Publish("grlx.sprouts.sprout-listener-test.facts", data); err != nil {
		t.Fatalf("publish: %v", err)
	}
	nc.Flush()
	time.Sleep(100 * time.Millisecond)

	if got := props.GetStringProp("sprout-listener-test", "os"); got != "linux" {
		t.Errorf("expected os=linux, got %q", got)
	}
	if got := props.GetStringProp("sprout-listener-test", "hostname"); got != "listener-test-host" {
		t.Errorf("expected hostname=listener-test-host, got %q", got)
	}
	if got := props.GetStringProp("sprout-listener-test", "num_cpu"); got != "4" {
		t.Errorf("expected num_cpu=4, got %q", got)
	}
}

func TestRegisterFarmerListener_EmptySproutID(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	RegisterFarmerListener(nc)
	nc.Flush()

	sf := SystemFacts{
		OS:       "linux",
		Arch:     "amd64",
		Hostname: "empty-id-host",
		SproutID: "",
	}
	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Should log error and not store — no panic.
	if err := nc.Publish("grlx.sprouts.unknown.facts", data); err != nil {
		t.Fatalf("publish: %v", err)
	}
	nc.Flush()
	time.Sleep(100 * time.Millisecond)

	// Props should not have been stored for empty sprout ID.
	if got := props.GetStringProp("", "os"); got != "" {
		t.Errorf("expected empty prop for empty sprout ID, got %q", got)
	}
}

func TestRegisterFarmerListener_InvalidJSON(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	RegisterFarmerListener(nc)
	nc.Flush()

	// Publish invalid JSON — should not panic.
	if err := nc.Publish("grlx.sprouts.bad.facts", []byte("not json")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	nc.Flush()
	time.Sleep(100 * time.Millisecond)
	// No assertion needed — just verifying no panic.
}

func TestRegisterFarmerListener_MultipleSprouts(t *testing.T) {
	nc, cleanup := startTestNATS(t)
	defer cleanup()

	RegisterFarmerListener(nc)
	nc.Flush()

	for _, sprout := range []struct {
		id       string
		os       string
		hostname string
	}{
		{"sprout-a", "linux", "host-a"},
		{"sprout-b", "darwin", "host-b"},
		{"sprout-c", "freebsd", "host-c"},
	} {
		sf := SystemFacts{
			OS:       sprout.os,
			Hostname: sprout.hostname,
			SproutID: sprout.id,
		}
		data, _ := json.Marshal(sf)
		if err := nc.Publish("grlx.sprouts."+sprout.id+".facts", data); err != nil {
			t.Fatalf("publish %s: %v", sprout.id, err)
		}
	}
	nc.Flush()
	time.Sleep(150 * time.Millisecond)

	if got := props.GetStringProp("sprout-a", "os"); got != "linux" {
		t.Errorf("sprout-a os: expected linux, got %q", got)
	}
	if got := props.GetStringProp("sprout-b", "os"); got != "darwin" {
		t.Errorf("sprout-b os: expected darwin, got %q", got)
	}
	if got := props.GetStringProp("sprout-c", "hostname"); got != "host-c" {
		t.Errorf("sprout-c hostname: expected host-c, got %q", got)
	}
}
