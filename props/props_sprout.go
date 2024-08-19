package props

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/gogrlx/grlx/pki"
)

var propFileLock sync.Mutex

// TODO: finalize and export this function

func saveToDiskFunc() error {
	propFileLock.Lock()
	defer propFileLock.Unlock()
	return saveToDisk()
}

func saveToDisk() error {
	// save to disk
	f, err := os.Create("/etc/grlx/props")
	if err != nil {
		return err
	}
	defer f.Close()
	sproutID := pki.GetSproutID()
	propMap := propCache[sproutID]
	jErr := json.NewEncoder(f).Encode(propMap)
	if jErr != nil {
		return jErr
	}
	return nil
}

// TODO: finalize and export this function
func loadFromDiskFunc() error {
	propFileLock.Lock()
	defer propFileLock.Unlock()
	return loadFromDisk()
}

func loadFromDisk() error {
	// load from disk
	_, err := os.Stat("/etc/grlx/props")
	if os.IsNotExist(err) {
		_, err := os.Create("/etc/grlx/props")
		if err != nil {
			return err
		}
		propCache = make(map[string]map[string]expProp)
		propCache[pki.GetSproutID()] = make(map[string]expProp)
		return nil
	}
	if err != nil {
		return err
	}
	f, err := os.Open("/etc/grlx/props")
	if err != nil {
		return err
	}
	defer f.Close()
	propCache = make(map[string]map[string]expProp)
	sproutID := pki.GetSproutID()
	propCache[sproutID] = make(map[string]expProp)
	propMap := propCache[sproutID]
	jErr := json.NewDecoder(f).Decode(&propMap)
	if jErr != nil {
		return jErr
	}
	propCache[sproutID] = propMap
	return nil
}
