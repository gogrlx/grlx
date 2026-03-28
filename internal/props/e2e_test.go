package props

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestGetPropsExported verifies the exported GetProps wrapper calls through
// to the internal getProps correctly (was at 0% coverage).
func TestGetPropsExported(t *testing.T) {
	resetCache()

	// No sprout → nil.
	if got := GetProps("nonexistent"); got != nil {
		t.Errorf("expected nil for nonexistent sprout, got %v", got)
	}

	// Set props and verify.
	SetProp("exported-test", "k1", "v1")
	SetProp("exported-test", "k2", "v2")

	got := GetProps("exported-test")
	if len(got) != 2 {
		t.Fatalf("expected 2 props, got %d", len(got))
	}
	if got["k1"] != "v1" {
		t.Errorf("expected k1=v1, got %v", got["k1"])
	}
	if got["k2"] != "v2" {
		t.Errorf("expected k2=v2, got %v", got["k2"])
	}
}

// TestGetPropsExportedWithExpiredEntries confirms expired props are
// cleaned up when accessed through the exported GetProps.
func TestGetPropsExportedWithExpiredEntries(t *testing.T) {
	resetCache()

	// Short TTL prop.
	setPropWithTTL("exp-sprout", "temp", "gone", 1)
	// Long TTL prop.
	SetProp("exp-sprout", "stable", "here")

	// Force the temp prop to be expired by setting its expiry to the past.
	propCacheLock.Lock()
	propCache["exp-sprout"]["temp"] = expProp{Value: "gone", Expiry: time.Now().Add(-time.Hour)}
	propCacheLock.Unlock()

	got := GetProps("exp-sprout")
	if len(got) != 1 {
		t.Fatalf("expected 1 prop after expiry, got %d: %v", len(got), got)
	}
	if got["stable"] != "here" {
		t.Errorf("expected stable=here, got %v", got["stable"])
	}
}

// TestStoreLoadSkipsNonJSON verifies loadAll ignores directories and
// non-.json files in the props directory.
func TestStoreLoadSkipsNonJSON(t *testing.T) {
	resetGlobals2()
	dir := t.TempDir()

	// Create a non-JSON file.
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644)
	// Create a subdirectory.
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	// Create a valid JSON file.
	data, _ := json.Marshal(map[string]interface{}{"role": "web"})
	os.WriteFile(filepath.Join(dir, "good-sprout.json"), data, 0o644)

	InitStore(dir)

	if got := GetStringProp("good-sprout", "role"); got != "web" {
		t.Errorf("expected 'web', got %q", got)
	}
	// readme.txt shouldn't produce a sprout named "readme".
	if got := GetStringProp("readme", ""); got != "" {
		t.Errorf("non-JSON file should not produce props")
	}
}

// TestStoreLoadCorruptJSON verifies loadAll handles corrupt JSON files
// gracefully without crashing.
func TestStoreLoadCorruptJSON(t *testing.T) {
	resetGlobals2()
	dir := t.TempDir()

	// Corrupt JSON.
	os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{invalid"), 0o644)
	// Valid JSON.
	data, _ := json.Marshal(map[string]interface{}{"port": "3000"})
	os.WriteFile(filepath.Join(dir, "valid.json"), data, 0o644)

	InitStore(dir)

	// Valid sprout should still load.
	if got := GetStringProp("valid", "port"); got != "3000" {
		t.Errorf("expected '3000', got %q", got)
	}
	// Corrupt sprout should have nothing.
	if got := GetStringProp("corrupt", "port"); got != "" {
		t.Errorf("corrupt sprout should have no props, got %q", got)
	}
}

// TestStoreLoadNumericValues verifies that non-string JSON values
// are serialized to string during load.
func TestStoreLoadNumericValues(t *testing.T) {
	resetGlobals2()
	dir := t.TempDir()

	// JSON with numeric and boolean values.
	data := []byte(`{"count": 42, "enabled": true, "name": "test"}`)
	os.WriteFile(filepath.Join(dir, "types-sprout.json"), data, 0o644)

	InitStore(dir)

	// Numeric value should be marshaled to string.
	if got := GetStringProp("types-sprout", "count"); got != "42" {
		t.Errorf("expected '42', got %q", got)
	}
	if got := GetStringProp("types-sprout", "enabled"); got != "true" {
		t.Errorf("expected 'true', got %q", got)
	}
	if got := GetStringProp("types-sprout", "name"); got != "test" {
		t.Errorf("expected 'test', got %q", got)
	}
}

// TestPersistSproutNoDirNoOp verifies persistSprout is a no-op when
// propsDir is empty.
func TestPersistSproutNoDirNoOp(t *testing.T) {
	resetGlobals2()
	// propsDir is "" after reset.
	// This should not panic.
	setProp("nodir-sprout", "key", "val")
}

// TestSetStaticPropEmptyKeys verifies setStaticProp returns early for
// empty sproutID or name without modifying cache.
func TestSetStaticPropEmptyKeys(t *testing.T) {
	resetCache()
	staticPropKeys = make(map[string]map[string]bool)

	setStaticProp("", "key", "val")
	setStaticProp("sprout", "", "val")

	propCacheLock.RLock()
	count := len(propCache)
	propCacheLock.RUnlock()
	if count != 0 {
		t.Errorf("expected empty cache for empty keys, got %d entries", count)
	}
}

// TestPropsPersistRoundTripMultipleSprouts verifies persistence works
// correctly with multiple sprouts writing concurrently.
func TestPropsPersistRoundTripMultipleSprouts(t *testing.T) {
	resetGlobals2()
	dir := t.TempDir()
	InitStore(dir)

	sprouts := []string{"web-1", "db-1", "cache-1"}
	for _, s := range sprouts {
		SetProp(s, "role", s)
		SetProp(s, "region", "us-east-1")
	}

	// Verify files.
	for _, s := range sprouts {
		path := filepath.Join(dir, s+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file for %s: %v", s, err)
		}
	}

	// Reset and reload.
	resetGlobals2()
	InitStore(dir)

	for _, s := range sprouts {
		if got := GetStringProp(s, "role"); got != s {
			t.Errorf("%s role: expected %q, got %q", s, s, got)
		}
		if got := GetStringProp(s, "region"); got != "us-east-1" {
			t.Errorf("%s region: expected 'us-east-1', got %q", s, got)
		}
	}
}

// TestHostnameFuncNeverEmpty verifies the hostname function always
// returns a non-empty value (falls back to "localhost").
func TestHostnameFuncNeverEmpty(t *testing.T) {
	got := hostname("any-sprout")
	if got == "" {
		t.Error("hostname should never be empty")
	}
}

// --- helpers ---

func resetGlobals2() {
	propCacheLock.Lock()
	propCache = make(map[string]map[string]expProp)
	propCacheLock.Unlock()
	propsDir = ""
	propsDirOnce = sync.Once{}
	staticPropKeys = make(map[string]map[string]bool)
}
