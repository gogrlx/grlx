package service

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gogrlx/grlx/config"
	"github.com/gogrlx/grlx/types"
)

var (
	provTex sync.Mutex
	provMap map[string]types.ServiceProvider

	ErrUnknownInit   = errors.New("unknown init system")
	ErrDuplicateInit = errors.New("provider for init system already initilaized")
	Init             string
)

func init() {
	provMap = make(map[string]types.ServiceProvider)
}

func RegisterProvider(provider types.ServiceProvider) error {
	provTex.Lock()
	defer provTex.Unlock()
	var err error
	init := provider.InitName()
	if _, ok := provMap[init]; !ok {
		provMap[init] = provider
	} else {
		err = errors.Join(err, fmt.Errorf("protocol %s already registered", init), ErrDuplicateInit)
	}
	return err
}

func guessInit() string {
	if Init != "" {
		return Init
	}
	// if the init system is specified in the config, use that
	if c := config.Init(); c != "" {
		Init = c
		return c
	}
	// allow the registered providers to guess the init system
	for _, initSys := range provMap {
		if initSys.IsInit() {
			Init = initSys.InitName()
			return Init
		}
	}
	return ""
}

func NewServiceProvider(id string, method string, params map[string]interface{}) (types.ServiceProvider, error) {
	provTex.Lock()
	defer provTex.Unlock()
	provider, ok := provMap[guessInit()]
	if !ok {
		return nil, ErrUnknownInit
	}
	return provider.Parse(id, method, params)
}
