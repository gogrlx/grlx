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
	// return "props"
	if propCache[sproutID] == nil {
		// get from sprout
		return ""
	}
	if propCache[sproutID][name] == (expProp{}) {
		// get from sprout
		return ""
	}
	if propCache[sproutID][name].Expiry.Before(time.Now()) {
		// get from sprout
		delete(propCache[sproutID], name)
		return ""
	}
	return fmt.Sprintf("%v", propCache[sproutID][name])
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
	if propCache[sproutID] == nil {
		// get from sprout
		return nil
	}
	props := make(map[string]interface{})
	for k, v := range propCache[sproutID] {
		if v.Expiry.Before(time.Now()) {
			// get from sprout
			delete(propCache[sproutID], k)
			continue
		}
		props[k] = v.Value
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
