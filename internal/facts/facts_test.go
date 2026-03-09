package facts

import (
	"runtime"
	"testing"
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
