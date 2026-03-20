package facts

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/props"
)

func TestCollect(t *testing.T) {
	f := Collect()

	if f.OS != runtime.GOOS {
		t.Errorf("expected OS=%s, got %s", runtime.GOOS, f.OS)
	}
	if f.Arch != runtime.GOARCH {
		t.Errorf("expected Arch=%s, got %s", runtime.GOARCH, f.Arch)
	}
	if f.Hostname == "" {
		t.Error("expected non-empty hostname")
	}
	if f.NumCPU < 1 {
		t.Errorf("expected NumCPU >= 1, got %d", f.NumCPU)
	}
	if f.GoVersion == "" {
		t.Error("expected non-empty GoVersion")
	}
	if f.KernelArch != runtime.GOARCH {
		t.Errorf("expected KernelArch=%s, got %s", runtime.GOARCH, f.KernelArch)
	}
}

func TestCollectSproutIDEmpty(t *testing.T) {
	f := Collect()
	if f.SproutID != "" {
		t.Errorf("expected empty SproutID from Collect(), got %q", f.SproutID)
	}
}

func TestCollectIPAddresses(t *testing.T) {
	f := Collect()
	// IPs come from localIPs — verify they match
	directIPs := localIPs()
	if len(f.IPAddresses) != len(directIPs) {
		t.Errorf("expected %d IPs, got %d", len(directIPs), len(f.IPAddresses))
	}
}

func TestLocalIPs(t *testing.T) {
	ips := localIPs()
	// Should have at least one non-loopback IP on most machines.
	if len(ips) == 0 {
		t.Skip("no non-loopback IPs found (containerized environment?)")
	}
	for _, ip := range ips {
		if ip == "127.0.0.1" || ip == "::1" {
			t.Errorf("loopback address %s should not be in localIPs", ip)
		}
	}
}

func TestSystemFactsJSON(t *testing.T) {
	original := SystemFacts{
		OS:          "linux",
		Arch:        "amd64",
		Hostname:    "test-sprout",
		GoVersion:   "go1.22.0",
		NumCPU:      4,
		IPAddresses: []string{"192.168.1.10", "10.0.0.5"},
		KernelArch:  "amd64",
		SproutID:    "sprout-abc",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal SystemFacts: %v", err)
	}

	var decoded SystemFacts
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SystemFacts: %v", err)
	}

	if decoded.OS != original.OS {
		t.Errorf("OS: expected %q, got %q", original.OS, decoded.OS)
	}
	if decoded.Arch != original.Arch {
		t.Errorf("Arch: expected %q, got %q", original.Arch, decoded.Arch)
	}
	if decoded.Hostname != original.Hostname {
		t.Errorf("Hostname: expected %q, got %q", original.Hostname, decoded.Hostname)
	}
	if decoded.GoVersion != original.GoVersion {
		t.Errorf("GoVersion: expected %q, got %q", original.GoVersion, decoded.GoVersion)
	}
	if decoded.NumCPU != original.NumCPU {
		t.Errorf("NumCPU: expected %d, got %d", original.NumCPU, decoded.NumCPU)
	}
	if decoded.SproutID != original.SproutID {
		t.Errorf("SproutID: expected %q, got %q", original.SproutID, decoded.SproutID)
	}
	if len(decoded.IPAddresses) != len(original.IPAddresses) {
		t.Fatalf("IPAddresses: expected %d, got %d", len(original.IPAddresses), len(decoded.IPAddresses))
	}
	for idx, ip := range decoded.IPAddresses {
		if ip != original.IPAddresses[idx] {
			t.Errorf("IPAddresses[%d]: expected %q, got %q", idx, original.IPAddresses[idx], ip)
		}
	}
}

func TestSystemFactsJSONOmitEmptySproutID(t *testing.T) {
	sf := SystemFacts{
		OS:       "linux",
		Arch:     "amd64",
		Hostname: "test",
	}
	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, exists := raw["sprout_id"]; exists {
		t.Error("expected sprout_id to be omitted when empty")
	}
}

func TestStoreFacts(t *testing.T) {
	sf := SystemFacts{
		OS:          "linux",
		Arch:        "arm64",
		Hostname:    "test-sprout-01",
		GoVersion:   "go1.22.0",
		NumCPU:      8,
		IPAddresses: []string{"10.0.0.1", "fd00::1"},
		KernelArch:  "arm64",
		SproutID:    "sprout-test-store",
	}

	storeFacts(sf)

	// Verify each fact was written to the props store.
	checks := map[string]string{
		"os":         "linux",
		"arch":       "arm64",
		"hostname":   "test-sprout-01",
		"go_version": "go1.22.0",
		"num_cpu":    "8",
	}
	for key, expected := range checks {
		got := props.GetStringProp(sf.SproutID, key)
		if got != expected {
			t.Errorf("prop %q: expected %q, got %q", key, expected, got)
		}
	}

	// Verify IP addresses stored as JSON array.
	ipsRaw := props.GetStringProp(sf.SproutID, "ip_addresses")
	var ips []string
	if err := json.Unmarshal([]byte(ipsRaw), &ips); err != nil {
		t.Fatalf("failed to unmarshal stored IPs: %v", err)
	}
	if len(ips) != 2 || ips[0] != "10.0.0.1" || ips[1] != "fd00::1" {
		t.Errorf("expected [10.0.0.1, fd00::1], got %v", ips)
	}
}

func TestStoreFactsNoIPs(t *testing.T) {
	sf := SystemFacts{
		OS:          "freebsd",
		Arch:        "amd64",
		Hostname:    "test-no-ips",
		GoVersion:   "go1.22.0",
		NumCPU:      2,
		IPAddresses: nil,
		SproutID:    "sprout-no-ips",
	}

	storeFacts(sf)

	// Core facts should still be stored.
	got := props.GetStringProp(sf.SproutID, "os")
	if got != "freebsd" {
		t.Errorf("expected os=freebsd, got %q", got)
	}

	// ip_addresses should not have been set (empty slice).
	ipsRaw := props.GetStringProp(sf.SproutID, "ip_addresses")
	if ipsRaw != "" {
		t.Errorf("expected empty ip_addresses for nil slice, got %q", ipsRaw)
	}
}

func TestStoreFactsOverwrite(t *testing.T) {
	sproutID := "sprout-overwrite-test"

	// Store initial facts.
	sf1 := SystemFacts{
		OS:       "linux",
		Arch:     "amd64",
		Hostname: "host-v1",
		NumCPU:   4,
		SproutID: sproutID,
	}
	storeFacts(sf1)

	if got := props.GetStringProp(sproutID, "hostname"); got != "host-v1" {
		t.Fatalf("initial hostname: expected host-v1, got %q", got)
	}

	// Store updated facts (simulates sprout reconnection).
	sf2 := SystemFacts{
		OS:       "linux",
		Arch:     "amd64",
		Hostname: "host-v2",
		NumCPU:   8,
		SproutID: sproutID,
	}
	storeFacts(sf2)

	if got := props.GetStringProp(sproutID, "hostname"); got != "host-v2" {
		t.Errorf("updated hostname: expected host-v2, got %q", got)
	}
	if got := props.GetStringProp(sproutID, "num_cpu"); got != "8" {
		t.Errorf("updated num_cpu: expected 8, got %q", got)
	}
}
