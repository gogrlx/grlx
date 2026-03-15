//go:build linux

package systemd

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestSystemdInitName(t *testing.T) {
	s := SystemdService{}
	if name := s.InitName(); name != "systemd" {
		t.Errorf("InitName() = %q, want %q", name, "systemd")
	}
}

func TestSystemdParseMissingName(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestSystemdParseNilProperties(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", nil)
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for nil properties, got %v", err)
	}
}

func TestSystemdParseEmptyName(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": ""})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for empty name, got %v", err)
	}
}

func TestSystemdParseNameNotString(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": 123})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for non-string name, got %v", err)
	}
}

func TestSystemdParseValid(t *testing.T) {
	s := SystemdService{}
	provider, err := s.Parse("svc-1", "running", map[string]interface{}{
		"name": "nginx",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	sd, ok := provider.(SystemdService)
	if !ok {
		t.Fatal("Parse() did not return SystemdService")
	}
	if sd.id != "svc-1" {
		t.Errorf("id = %q, want %q", sd.id, "svc-1")
	}
	if sd.name != "nginx" {
		t.Errorf("name = %q, want %q", sd.name, "nginx")
	}
	if sd.method != "running" {
		t.Errorf("method = %q, want %q", sd.method, "running")
	}
	if sd.userMode {
		t.Error("userMode should default to false")
	}
}

func TestSystemdParseUserMode(t *testing.T) {
	s := SystemdService{}
	provider, err := s.Parse("svc-1", "enabled", map[string]interface{}{
		"name":     "podman",
		"userMode": true,
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	sd := provider.(SystemdService)
	if !sd.userMode {
		t.Error("userMode should be true when set in properties")
	}
}

func TestSystemdParseUserModeNonBool(t *testing.T) {
	s := SystemdService{}
	provider, err := s.Parse("svc-1", "enabled", map[string]interface{}{
		"name":     "podman",
		"userMode": "yes",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	sd := provider.(SystemdService)
	if sd.userMode {
		t.Error("userMode should be false when set to non-bool value")
	}
}

func TestSystemdProperties(t *testing.T) {
	props := map[string]interface{}{"name": "nginx", "userMode": false}
	s := SystemdService{props: props}
	got, err := s.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if got["name"] != "nginx" {
		t.Errorf("Properties()[name] = %v, want %q", got["name"], "nginx")
	}
}
