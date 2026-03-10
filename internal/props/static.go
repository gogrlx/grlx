package props

import (
	"fmt"
	"time"

	log "github.com/gogrlx/grlx/v2/internal/log"
)

// StaticPropTTL is the TTL for static props loaded from config.
// Set to ~100 years so they effectively never expire.
const StaticPropTTL = 100 * 365 * 24 * time.Hour

// staticPropKeys tracks which props were loaded from config so they
// can be refreshed on config reload without losing dynamic props.
var staticPropKeys = make(map[string]map[string]bool)

// LoadStaticProps loads static properties from the farmer config.
// Props are organized by sprout ID with key-value pairs.
//
// Expected config structure:
//
//	props:
//	  static:
//	    <sproutID>:
//	      key1: value1
//	      key2: value2
//
// Static props use a very long TTL and are not persisted to disk
// (they come from config, not runtime state).
func LoadStaticProps(staticCfg map[string]interface{}) {
	if staticCfg == nil {
		return
	}

	loaded := 0
	for sproutID, propsI := range staticCfg {
		propsMap, ok := propsI.(map[string]interface{})
		if !ok {
			log.Errorf("props: static props for sprout %q is not a map", sproutID)
			continue
		}
		if staticPropKeys[sproutID] == nil {
			staticPropKeys[sproutID] = make(map[string]bool)
		}
		for k, v := range propsMap {
			strVal := fmt.Sprintf("%v", v)
			setStaticProp(sproutID, k, strVal)
			staticPropKeys[sproutID][k] = true
			loaded++
		}
	}

	if loaded > 0 {
		log.Noticef("props: loaded %d static prop(s) from config", loaded)
	}
}

// setStaticProp sets a prop with the static TTL. Static props are not
// persisted to disk since they originate from config.
func setStaticProp(sproutID, name, value string) {
	if sproutID == "" || name == "" {
		return
	}
	propCacheLock.Lock()
	if propCache[sproutID] == nil {
		propCache[sproutID] = make(map[string]expProp)
	}
	propCache[sproutID][name] = expProp{
		Value:  value,
		Expiry: time.Now().Add(StaticPropTTL),
	}
	propCacheLock.Unlock()
	// Do NOT persist — static props come from config, not runtime.
}

// IsStaticProp returns true if the given prop was loaded from config.
func IsStaticProp(sproutID, name string) bool {
	keys, ok := staticPropKeys[sproutID]
	if !ok {
		return false
	}
	return keys[name]
}

// ClearStaticProps removes all previously loaded static props from cache.
// Called before reloading config to avoid stale static props.
func ClearStaticProps() {
	propCacheLock.Lock()
	for sproutID, keys := range staticPropKeys {
		for k := range keys {
			if sproutProps, ok := propCache[sproutID]; ok {
				delete(sproutProps, k)
			}
		}
	}
	propCacheLock.Unlock()
	staticPropKeys = make(map[string]map[string]bool)
}
