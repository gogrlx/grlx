package service

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/types"
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
	// Check if the init system is systemd
	// https://manpages.ubuntu.com/manpages/xenial/en/man3/sd_booted.3.html
	if _, ok := os.Stat("/run/systemd/system"); ok == nil {
		return "systemd"
	}

	for _, initSys := range provMap {
		if initSys.IsInit() {
			Init = initSys.InitName()
			return Init
		}
	}
	// otherwise, return the name of the process in PID 1
	f, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return "unknown"
	}
	return string(f)
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
