package props

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

// propsDir is the directory where props are persisted as JSON files.
// Each sprout gets its own file: <propsDir>/<sproutID>.json.
var (
	propsDir     string
	propsDirOnce sync.Once
)

// InitStore sets the directory for persistent props storage and loads
// any existing props from disk into the in-memory cache.
func InitStore(dir string) {
	propsDirOnce.Do(func() {
		propsDir = dir
		if err := os.MkdirAll(propsDir, 0o700); err != nil {
			log.Errorf("props: failed to create store dir %s: %v", propsDir, err)
			return
		}
		loadAll()
	})
}

// persistSprout writes all current (non-expired) props for a sprout to disk.
func persistSprout(sproutID string) {
	if propsDir == "" {
		return
	}

	current := getProps(sproutID)
	if len(current) == 0 {
		// Remove the file if no props remain.
		path := filepath.Join(propsDir, sproutID+".json")
		os.Remove(path)
		return
	}

	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		log.Errorf("props: failed to marshal props for %s: %v", sproutID, err)
		return
	}

	path := filepath.Join(propsDir, sproutID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Errorf("props: failed to write props for %s: %v", sproutID, err)
	}
}

// loadAll reads all sprout JSON files from disk and populates the cache.
// Props loaded from disk get the default TTL.
func loadAll() {
	entries, err := os.ReadDir(propsDir)
	if err != nil {
		log.Errorf("props: failed to read store dir: %v", err)
		return
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		sproutID := name[:len(name)-5] // strip .json

		data, readErr := os.ReadFile(filepath.Join(propsDir, name))
		if readErr != nil {
			log.Errorf("props: failed to read %s: %v", name, readErr)
			continue
		}

		var kv map[string]interface{}
		if unmarshalErr := json.Unmarshal(data, &kv); unmarshalErr != nil {
			log.Errorf("props: failed to parse %s: %v", name, unmarshalErr)
			continue
		}

		for k, v := range kv {
			// Convert value to string for storage.
			var strVal string
			switch tv := v.(type) {
			case string:
				strVal = tv
			default:
				b, _ := json.Marshal(tv)
				strVal = string(b)
			}
			setPropWithTTL(sproutID, k, strVal, DefaultPropTTL)
		}
		loaded++
	}

	if loaded > 0 {
		log.Noticef("props: loaded persistent props for %d sprout(s)", loaded)
	}
}
