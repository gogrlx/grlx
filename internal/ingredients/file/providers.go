package file

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/taigrr/log-socket/log"
)

var (
	provTex sync.Mutex
	provMap map[string]FileProvider

	ErrUnknownProtocol   = errors.New("unknown protocol")
	ErrUnknownMethod     = errors.New("unknown method")
	ErrDuplicateProtocol = errors.New("duplicate protocol")
)

func RegisterProvider(provider FileProvider) error {
	provTex.Lock()
	defer provTex.Unlock()
	var err error
	methods := provider.Protocols()
	for _, method := range methods {
		if method == "" {
			err = errors.Join(err, fmt.Errorf("cannot register empty protocol"))
			continue
		}
		// don't override existing protocol handlers
		_, ok := provMap[method]
		if !ok {
			provMap[method] = provider
		} else {
			err = errors.Join(err, fmt.Errorf("protocol %s already registered", method), ErrDuplicateProtocol)
		}
	}
	return err
}

func guessProtocol(source string) string {
	if strings.HasPrefix(source, "/") {
		log.Tracef("guessing protocol %s for source %s", "file", source)
		return "file"
	}
	if strings.Contains(source, "://") {
		proto := strings.Split(source, "://")[0]
		log.Tracef("guessing protocol %s for source %s", proto, source)
		return proto
	}
	log.Tracef("unknown protocol for source %s", source)
	return ""
}

func NewFileProvider(id string, source, destination, hash string, params map[string]interface{}) (FileProvider, error) {
	provTex.Lock()
	defer provTex.Unlock()
	protocol := guessProtocol(source)
	r, ok := provMap[protocol]
	if !ok {
		return nil, errors.Join(ErrUnknownProtocol, fmt.Errorf("unknown protocol: %s", protocol))
	}
	return r.Parse(id, source, destination, hash, params)
}
