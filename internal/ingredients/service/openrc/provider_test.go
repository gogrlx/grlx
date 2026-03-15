//go:build linux

package openrc

import (
	"context"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestOpenRCInitName(t *testing.T) {
	s := OpenRCService{}
	if name := s.InitName(); name != "openrc" {
		t.Errorf("InitName() = %q, want %q", name, "openrc")
	}
}

func TestOpenRCParseMissingName(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestOpenRCParseNilProperties(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", nil)
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for nil properties, got %v", err)
	}
}

func TestOpenRCParseEmptyName(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": ""})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for empty name, got %v", err)
	}
}

func TestOpenRCParseNameNotString(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": 42})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for non-string name, got %v", err)
	}
}

func TestOpenRCParseValid(t *testing.T) {
	s := OpenRCService{}
	provider, err := s.Parse("svc-1", "running", map[string]interface{}{
		"name": "nginx",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	orc, ok := provider.(OpenRCService)
	if !ok {
		t.Fatal("Parse() did not return OpenRCService")
	}
	if orc.id != "svc-1" {
		t.Errorf("id = %q, want %q", orc.id, "svc-1")
	}
	if orc.name != "nginx" {
		t.Errorf("name = %q, want %q", orc.name, "nginx")
	}
	if orc.method != "running" {
		t.Errorf("method = %q, want %q", orc.method, "running")
	}
}

func TestOpenRCParseWithRunlevel(t *testing.T) {
	s := OpenRCService{}
	provider, err := s.Parse("svc-1", "enabled", map[string]interface{}{
		"name":     "sshd",
		"runlevel": "default",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	orc := provider.(OpenRCService)
	if orc.opts.Runlevel != "default" {
		t.Errorf("opts.Runlevel = %q, want %q", orc.opts.Runlevel, "default")
	}
}

func TestOpenRCProperties(t *testing.T) {
	props := map[string]interface{}{"name": "nginx"}
	s := OpenRCService{props: props}
	got, err := s.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if got["name"] != "nginx" {
		t.Errorf("Properties()[name] = %v, want %q", got["name"], "nginx")
	}
}

func TestOpenRCMaskNotSupported(t *testing.T) {
	s := OpenRCService{name: "nginx"}
	ctx := context.Background()

	if err := s.Mask(ctx); err != ErrMaskNotSupported {
		t.Errorf("Mask() = %v, want ErrMaskNotSupported", err)
	}
	if err := s.Unmask(ctx); err != ErrMaskNotSupported {
		t.Errorf("Unmask() = %v, want ErrMaskNotSupported", err)
	}
}

func TestOpenRCIsMaskedAlwaysFalse(t *testing.T) {
	s := OpenRCService{name: "nginx"}
	masked, err := s.IsMasked(context.Background())
	if err != nil {
		t.Fatalf("IsMasked() error: %v", err)
	}
	if masked {
		t.Error("IsMasked() should always return false for OpenRC")
	}
}
