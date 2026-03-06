package service

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/gogrlx/grlx/v2/internal/config"
)

var (
	provTex sync.Mutex
	provMap map[string]ServiceProvider

	ErrUnknownInit   = errors.New("unknown init system")
	ErrDuplicateInit = errors.New("provider for init system already initialized")
	Init             string
)

func init() {
	provMap = make(map[string]ServiceProvider)
}

func RegisterProvider(provider ServiceProvider) error {
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
	// Check all registered providers (systemd, rcd, etc.) via their IsInit probe.
	for _, initSys := range provMap {
		if initSys.IsInit() {
			Init = initSys.InitName()
			return Init
		}
	}
	// Fallback: try reading the name of PID 1 (Linux procfs).
	if f, err := os.ReadFile("/proc/1/comm"); err == nil {
		return string(f)
	}
	return "unknown"
}

func NewServiceProvider(id string, method string, params map[string]interface{}) (ServiceProvider, error) {
	provTex.Lock()
	defer provTex.Unlock()
	provider, ok := provMap[guessInit()]
	if !ok {
		return nil, ErrUnknownInit
	}
	return provider.Parse(id, method, params)
}
