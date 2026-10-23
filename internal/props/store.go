package props

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

// propsDir is the directory where props are persisted as JSON files.
// Each sprout gets its own file: <propsDir>/<sproutID>.json.
var (
	propsDir     string
	propsDirOnce sync.Once
	persistLock  sync.Mutex
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

	// Serialize the snapshot and file operation together so an older snapshot
	// cannot overwrite a newer one and concurrent writes cannot interleave.
	persistLock.Lock()
	defer persistLock.Unlock()

	current := getProps(sproutID)
	if len(current) == 0 {
		// Remove the file if no props remain.
		path := sproutPropsPath(sproutID)
		os.Remove(path)
		return
	}

	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		log.Errorf("props: failed to marshal props for %s: %v", sproutID, err)
		return
	}

	path := sproutPropsPath(sproutID)
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
		sproutID := sproutIDFromPropsFile(name)
		if canonical := sproutPropsPath(sproutID); filepath.Base(canonical) != name {
			legacy := filepath.Join(propsDir, name)
			if _, statErr := os.Stat(canonical); statErr == nil {
				log.Errorf("props: legacy file %s conflicts with %s; skipping", name, filepath.Base(canonical))
				continue
			} else if renameErr := os.Rename(legacy, canonical); renameErr != nil {
				log.Errorf("props: failed to migrate %s: %v", name, renameErr)
			} else {
				name = filepath.Base(canonical)
			}
		}

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

const propsFileExt = ".json"

func escapeSproutID(sproutID string) string {
	return strings.ReplaceAll(url.PathEscape(sproutID), `\`, "%5C")
}

func sproutPropsPath(sproutID string) string {
	return filepath.Join(propsDir, escapeSproutID(sproutID)+propsFileExt)
}

// sproutIDFromPropsFile decodes a props filename back into a sprout ID.
// Files written before IDs were escaped used the raw sprout ID, so names that
// do not round-trip through escaping are treated as legacy raw IDs.
func sproutIDFromPropsFile(name string) string {
	stem := strings.TrimSuffix(name, propsFileExt)
	sproutID, err := url.PathUnescape(stem)
	if err != nil || escapeSproutID(sproutID) != stem {
		return stem
	}
	return sproutID
}
