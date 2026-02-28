package pkg

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/snack"
)

func TestMethods(t *testing.T) {
	p := Pkg{}
	name, methods := p.Methods()
	if name != "pkg" {
		t.Errorf("expected ingredient name 'pkg', got %q", name)
	}
	expected := []string{
		"group_installed", "held", "installed", "latest",
		"purged", "removed", "unheld", "uptodate",
	}
	if len(methods) != len(expected) {
		t.Fatalf("expected %d methods, got %d", len(expected), len(methods))
	}
	for i, m := range methods {
		if m != expected[i] {
			t.Errorf("method[%d]: expected %q, got %q", i, expected[i], m)
		}
	}
}

func TestParseMissingName(t *testing.T) {
	p := Pkg{}
	_, err := p.Parse("test-id", "installed", map[string]interface{}{})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestParseInvalidMethod(t *testing.T) {
	p := Pkg{}
	_, err := p.Parse("test-id", "nonexistent", map[string]interface{}{"name": "nginx"})
	if err != ingredients.ErrInvalidMethod {
		t.Errorf("expected ErrInvalidMethod, got %v", err)
	}
}

func TestParseValidMethods(t *testing.T) {
	p := Pkg{}
	methods := []string{
		"installed", "latest", "removed", "purged",
		"uptodate", "held", "unheld", "group_installed",
	}
	for _, method := range methods {
		cooker, err := p.Parse("test-id", method, map[string]interface{}{"name": "nginx"})
		if err != nil {
			t.Errorf("Parse(%q) returned error: %v", method, err)
			continue
		}
		parsed, ok := cooker.(Pkg)
		if !ok {
			t.Errorf("Parse(%q) returned wrong type", method)
			continue
		}
		if parsed.name != "nginx" {
			t.Errorf("Parse(%q) name = %q, want %q", method, parsed.name, "nginx")
		}
		if parsed.method != method {
			t.Errorf("Parse(%q) method = %q, want %q", method, parsed.method, method)
		}
	}
}

func TestPropertiesForMethod(t *testing.T) {
	p := Pkg{}
	tests := []struct {
		method   string
		wantKeys []string
		wantErr  bool
	}{
		{"installed", []string{"name", "version", "fromrepo", "pkgs", "refresh", "reinstall"}, false},
		{"latest", []string{"name", "fromrepo", "pkgs", "refresh"}, false},
		{"removed", []string{"name", "pkgs"}, false},
		{"purged", []string{"name", "pkgs"}, false},
		{"uptodate", []string{"name", "refresh"}, false},
		{"held", []string{"name", "pkgs"}, false},
		{"unheld", []string{"name", "pkgs"}, false},
		{"group_installed", []string{"name"}, false},
		{"bogus", nil, true},
	}
	for _, tc := range tests {
		props, err := p.PropertiesForMethod(tc.method)
		if tc.wantErr {
			if err == nil {
				t.Errorf("PropertiesForMethod(%q): expected error", tc.method)
			}
			continue
		}
		if err != nil {
			t.Errorf("PropertiesForMethod(%q): unexpected error: %v", tc.method, err)
			continue
		}
		for _, key := range tc.wantKeys {
			if _, ok := props[key]; !ok {
				t.Errorf("PropertiesForMethod(%q): missing key %q", tc.method, key)
			}
		}
	}
}

func TestParsePkgsList(t *testing.T) {
	// Plain strings
	targets := parsePkgsList([]interface{}{"nginx", "curl", "vim"})
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	if targets[0].Name != "nginx" {
		t.Errorf("expected 'nginx', got %q", targets[0].Name)
	}

	// Map entries (version pinning)
	targets = parsePkgsList([]interface{}{
		"nginx",
		map[string]interface{}{"redis": ">=7.0"},
	})
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	if targets[1].Name != "redis" || targets[1].Version != ">=7.0" {
		t.Errorf("expected redis >=7.0, got %q %q", targets[1].Name, targets[1].Version)
	}

	// Invalid input
	targets = parsePkgsList("not-a-list")
	if targets != nil {
		t.Errorf("expected nil for invalid input, got %v", targets)
	}
}

func TestParseTargets(t *testing.T) {
	// Single package with version and repo
	p := Pkg{
		name: "nginx",
		properties: map[string]interface{}{
			"name":     "nginx",
			"version":  "1.24.0",
			"fromrepo": "stable",
		},
	}
	targets := p.parseTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "nginx" || targets[0].Version != "1.24.0" || targets[0].FromRepo != "stable" {
		t.Errorf("unexpected target: %+v", targets[0])
	}

	// With pkgs override
	p.properties["pkgs"] = []interface{}{"curl", "wget"}
	targets = p.parseTargets()
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
}

func TestGetBoolProp(t *testing.T) {
	props := map[string]interface{}{
		"refresh":   true,
		"reinstall": false,
		"badtype":   "yes",
	}
	if !getBoolProp(props, "refresh", false) {
		t.Error("expected true for 'refresh'")
	}
	if getBoolProp(props, "reinstall", true) {
		t.Error("expected false for 'reinstall'")
	}
	if getBoolProp(props, "missing", true) != true {
		t.Error("expected default true for missing key")
	}
	if getBoolProp(props, "badtype", false) != false {
		t.Error("expected default false for non-bool type")
	}
}

func TestBuildOptions(t *testing.T) {
	p := Pkg{
		properties: map[string]interface{}{
			"name":      "nginx",
			"refresh":   true,
			"fromrepo":  "stable",
			"reinstall": true,
		},
	}
	opts := p.buildOptions()
	// Apply and check
	o := snack.ApplyOptions(opts...)
	if !o.AssumeYes {
		t.Error("expected AssumeYes")
	}
	if !o.Refresh {
		t.Error("expected Refresh")
	}
	if o.FromRepo != "stable" {
		t.Errorf("expected FromRepo 'stable', got %q", o.FromRepo)
	}
	if !o.Reinstall {
		t.Error("expected Reinstall")
	}
}

func TestProperties(t *testing.T) {
	props := map[string]interface{}{"name": "nginx", "version": "1.0"}
	p := Pkg{properties: props}
	got, err := p.Properties()
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "nginx" || got["version"] != "1.0" {
		t.Errorf("unexpected properties: %v", got)
	}
}
