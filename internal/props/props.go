package props

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// DefaultPropTTL is the default time-to-live for cached properties.
const DefaultPropTTL = 5 * time.Minute

// ErrInvalidPropKey is returned when a sproutID or property name is empty.
var ErrInvalidPropKey = errors.New("sproutID and property name must not be empty")

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

// GetStringProp returns the string value of a single property for a sprout.
// Returns an empty string if the sprout or property does not exist, or if the
// property has expired.
func GetStringProp(sproutID, name string) string {
	return getStringProp(sproutID, name)
}

func getStringProp(sproutID, name string) string {
	propCacheLock.RLock()
	sproutProps, ok := propCache[sproutID]
	if !ok || sproutProps == nil {
		propCacheLock.RUnlock()
		return ""
	}
	prop, ok := sproutProps[name]
	if !ok || prop == (expProp{}) {
		propCacheLock.RUnlock()
		return ""
	}
	if prop.Expiry.Before(time.Now()) {
		propCacheLock.RUnlock()
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

// SetProp sets a property for a sprout with the default TTL.
func SetProp(sproutID, name, value string) error {
	return setProp(sproutID, name, value)
}

func setProp(sproutID, name, value string) error {
	return setPropWithTTL(sproutID, name, value, DefaultPropTTL)
}

func setPropWithTTL(sproutID, name, value string, ttl time.Duration) error {
	if sproutID == "" || name == "" {
		return ErrInvalidPropKey
	}
	propCacheLock.Lock()
	if propCache[sproutID] == nil {
		propCache[sproutID] = make(map[string]expProp)
	}
	propCache[sproutID][name] = expProp{
		Value:  value,
		Expiry: time.Now().Add(ttl),
	}
	propCacheLock.Unlock()
	persistSprout(sproutID)
	return nil
}

func GetDeletePropFunc(sproutID string) func(string) error {
	return func(name string) error {
		return deleteProp(sproutID, name)
	}
}

// DeleteProp removes a property for a sprout. Returns nil if the property
// does not exist.
func DeleteProp(sproutID, name string) error {
	return deleteProp(sproutID, name)
}

func deleteProp(sproutID, name string) error {
	if sproutID == "" || name == "" {
		return ErrInvalidPropKey
	}
	propCacheLock.Lock()
	sproutProps, ok := propCache[sproutID]
	if !ok || sproutProps == nil {
		propCacheLock.Unlock()
		return nil
	}
	delete(sproutProps, name)
	propCacheLock.Unlock()
	persistSprout(sproutID)
	return nil
}

func GetPropsFunc(sproutID string) func() map[string]interface{} {
	return func() map[string]interface{} {
		return getProps(sproutID)
	}
}

// GetProps returns all non-expired properties for a sprout. Returns nil if the
// sprout has no properties.
func GetProps(sproutID string) map[string]interface{} {
	return getProps(sproutID)
}

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
