package props

import (
	"testing"
)

func TestLoadStaticProps(t *testing.T) {
	// Reset cache state.
	propCacheLock.Lock()
	propCache = make(map[string]map[string]expProp)
	propCacheLock.Unlock()
	staticPropKeys = make(map[string]map[string]bool)

	cfg := map[string]interface{}{
		"web-1": map[string]interface{}{
			"role": "webserver",
			"env":  "production",
			"port": "8080",
		},
		"db-1": map[string]interface{}{
			"role": "database",
			"env":  "production",
		},
	}

	LoadStaticProps(cfg)

	// Verify props loaded.
	if got := GetStringProp("web-1", "role"); got != "webserver" {
		t.Errorf("web-1 role = %q, want %q", got, "webserver")
	}
	if got := GetStringProp("web-1", "env"); got != "production" {
		t.Errorf("web-1 env = %q, want %q", got, "production")
	}
	if got := GetStringProp("web-1", "port"); got != "8080" {
		t.Errorf("web-1 port = %q, want %q", got, "8080")
	}
	if got := GetStringProp("db-1", "role"); got != "database" {
		t.Errorf("db-1 role = %q, want %q", got, "database")
	}

	// Verify IsStaticProp.
	if !IsStaticProp("web-1", "role") {
		t.Error("web-1 role should be static")
	}
	if IsStaticProp("web-1", "nonexistent") {
		t.Error("nonexistent should not be static")
	}
	if IsStaticProp("unknown-sprout", "role") {
		t.Error("unknown sprout should not have static props")
	}
}

func TestLoadStaticPropsNil(t *testing.T) {
	// Should not panic.
	LoadStaticProps(nil)
}

func TestClearStaticProps(t *testing.T) {
	propCacheLock.Lock()
	propCache = make(map[string]map[string]expProp)
	propCacheLock.Unlock()
	staticPropKeys = make(map[string]map[string]bool)

	cfg := map[string]interface{}{
		"web-1": map[string]interface{}{
			"role": "webserver",
		},
	}
	LoadStaticProps(cfg)

	// Also set a dynamic prop.
	setPropWithTTL("web-1", "dynamic-key", "dynamic-val", DefaultPropTTL)

	// Clear static.
	ClearStaticProps()

	// Static prop should be gone.
	if got := GetStringProp("web-1", "role"); got != "" {
		t.Errorf("static prop role should be cleared, got %q", got)
	}
	// Dynamic prop should remain.
	if got := GetStringProp("web-1", "dynamic-key"); got != "dynamic-val" {
		t.Errorf("dynamic prop should remain, got %q", got)
	}
	// IsStaticProp should return false.
	if IsStaticProp("web-1", "role") {
		t.Error("role should no longer be static after clear")
	}
}

func TestStaticPropsNotPersisted(t *testing.T) {
	// Reset.
	propCacheLock.Lock()
	propCache = make(map[string]map[string]expProp)
	propCacheLock.Unlock()
	staticPropKeys = make(map[string]map[string]bool)
	// Ensure propsDir is empty so persistSprout is a no-op.
	propsDir = ""

	cfg := map[string]interface{}{
		"s1": map[string]interface{}{
			"key": "val",
		},
	}
	LoadStaticProps(cfg)

	// Verify prop is in cache.
	if got := GetStringProp("s1", "key"); got != "val" {
		t.Errorf("expected 'val', got %q", got)
	}
}

func TestLoadStaticPropsInvalidEntry(t *testing.T) {
	propCacheLock.Lock()
	propCache = make(map[string]map[string]expProp)
	propCacheLock.Unlock()
	staticPropKeys = make(map[string]map[string]bool)

	// "bad-sprout" has a non-map value — should be skipped without panic.
	cfg := map[string]interface{}{
		"good-sprout": map[string]interface{}{
			"key": "val",
		},
		"bad-sprout": "not-a-map",
	}
	LoadStaticProps(cfg)

	if got := GetStringProp("good-sprout", "key"); got != "val" {
		t.Errorf("good-sprout key = %q, want 'val'", got)
	}
	// bad-sprout should have nothing.
	if got := GetStringProp("bad-sprout", "key"); got != "" {
		t.Errorf("bad-sprout key should be empty, got %q", got)
	}
}
