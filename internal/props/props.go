package props

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type expProp struct {
	Value  interface{}
	Expiry time.Time
}

var (
	propCache     = make(map[string]map[string]expProp)
	propCacheLock = sync.RWMutex{}
)

func init() {
	propCache = make(map[string]map[string]expProp)
}

func GetStringPropFunc(sproutID string) func(string) string {
	return func(name string) string {
		return getStringProp(sproutID, name)
	}
}

// TODO: implement GetProp
func getStringProp(sproutID, name string) string {
	propCacheLock.RLock()
	sproutProps, ok := propCache[sproutID]
	if !ok || sproutProps == nil {
		propCacheLock.RUnlock()
		// get from sprout
		return ""
	}
	prop, ok := sproutProps[name]
	if !ok || prop == (expProp{}) {
		propCacheLock.RUnlock()
		// get from sprout
		return ""
	}
	if prop.Expiry.Before(time.Now()) {
		propCacheLock.RUnlock()
		// get from sprout — need write lock to delete
		propCacheLock.Lock()
		delete(propCache[sproutID], name)
		propCacheLock.Unlock()
		return ""
	}
	propCacheLock.RUnlock()
	return fmt.Sprintf("%v", prop.Value)
}

func SetPropFunc(sproutID string) func(string, string) error {
	return func(name, value string) error {
		return setProp(sproutID, name, value)
	}
}

// TODO: implement SetProp
func setProp(sproutID, name, value string) error {
	return nil
}

func GetDeletePropFunc(sproutID string) func(string) error {
	return func(name string) error {
		return deleteProp(sproutID, name)
	}
}

// TODO: implement DeleteProp
func deleteProp(sproutID, name string) error {
	return nil
}

func GetPropsFunc(sproutID string) func() map[string]interface{} {
	return func() map[string]interface{} {
		return getProps(sproutID)
	}
}

// TODO: implement getProps
func getProps(sproutID string) map[string]interface{} {
	propCacheLock.RLock()
	sproutProps, ok := propCache[sproutID]
	if !ok || sproutProps == nil {
		propCacheLock.RUnlock()
		// get from sprout
		return nil
	}
	propCacheLock.RUnlock()

	props := make(map[string]interface{})
	var expired []string
	propCacheLock.RLock()
	for k, v := range propCache[sproutID] {
		if v.Expiry.Before(time.Now()) {
			expired = append(expired, k)
			continue
		}
		props[k] = v.Value
	}
	propCacheLock.RUnlock()

	if len(expired) > 0 {
		propCacheLock.Lock()
		for _, k := range expired {
			delete(propCache[sproutID], k)
		}
		propCacheLock.Unlock()
	}
	return props
}

func GetHostnameFunc(sproutID string) func() string {
	return func() string {
		return hostname(sproutID)
	}
}

func hostname(sproutID string) string {
	hostname, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return hostname
}
