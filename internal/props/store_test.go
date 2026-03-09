package props

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// resetGlobals resets package-level state so tests are independent.
func resetGlobals() {
	propCacheLock.Lock()
	propCache = make(map[string]map[string]expProp)
	propCacheLock.Unlock()
	propsDir = ""
	propsDirOnce = sync.Once{}
}

func TestPersistAndLoad(t *testing.T) {
	resetGlobals()
	dir := t.TempDir()
	InitStore(dir)

	// Set some props.
	if err := SetProp("sprout-1", "os", "linux"); err != nil {
		t.Fatal(err)
	}
	if err := SetProp("sprout-1", "arch", "amd64"); err != nil {
		t.Fatal(err)
	}

	// Verify the file was written.
	path := filepath.Join(dir, "sprout-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected props file to exist: %v", err)
	}

	var kv map[string]interface{}
	if err := json.Unmarshal(data, &kv); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if kv["os"] != "linux" || kv["arch"] != "amd64" {
		t.Errorf("unexpected persisted values: %v", kv)
	}

	// Reset cache and reload from disk.
	resetGlobals()
	InitStore(dir)

	got := GetStringProp("sprout-1", "os")
	if got != "linux" {
		t.Errorf("expected 'linux' after reload, got %q", got)
	}
	got = GetStringProp("sprout-1", "arch")
	if got != "amd64" {
		t.Errorf("expected 'amd64' after reload, got %q", got)
	}
}

func TestDeleteRemovesFile(t *testing.T) {
	resetGlobals()
	dir := t.TempDir()
	InitStore(dir)

	SetProp("sprout-2", "role", "web")
	path := filepath.Join(dir, "sprout-2.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("expected file to exist after SetProp")
	}

	DeleteProp("sprout-2", "role")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be removed after deleting last prop")
	}
}
