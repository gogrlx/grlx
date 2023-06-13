package file

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gogrlx/grlx/types"
)

var (
	provTex sync.Mutex
	provMap map[string]types.FileProvider

	ErrUnknownProtocol   = errors.New("unknown protocol")
	ErrUnknownMethod     = errors.New("unknown method")
	ErrDuplicateProtocol = errors.New("duplicate protocol")
)

func init() {
	provMap = make(map[string]types.FileProvider)
}

func RegisterProvider(provider types.FileProvider) error {
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
		return "file"
	}
	if strings.Contains(source, "://") {
		return strings.Split(source, "://")[0]
	}
	return ""
}

func NewFileProvider(id string, source, destination, hash string, params map[string]interface{}) (types.FileProvider, error) {
	provTex.Lock()
	defer provTex.Unlock()
	protocol := guessProtocol(source)
	r, ok := provMap[protocol]
	if !ok {
		return nil, errors.Join(ErrUnknownProtocol, fmt.Errorf("unknown protocol: %s", protocol))
	}
	return r.Parse(id, source, destination, hash, params)
}
