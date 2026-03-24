//go:build linux

package systemd

import (
	"context"
	"os"

	"github.com/taigrr/systemctl"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/internal/ingredients/service"
)

// Function variables for systemctl operations — replaceable in tests.
// Functions from systemctl.go have variadic args, helpers.go functions don't.
var (
	sysStart   = systemctl.Start
	sysStop    = systemctl.Stop
	sysRestart = systemctl.Restart
	sysReload  = systemctl.Reload
	sysEnable  = systemctl.Enable
	sysDisable = systemctl.Disable
	sysMask    = systemctl.Mask
	sysUnmask  = systemctl.Unmask
	sysStatus  = systemctl.Status

	// These come from helpers.go — no variadic args.
	sysIsEnabled = systemctl.IsEnabled
	sysIsMasked  = systemctl.IsMasked
	sysIsRunning = systemctl.IsRunning
)

type SystemdService struct {
	id       string
	name     string
	method   string
	userMode bool
	props    map[string]interface{}
}

// Compile-time interface check.
var _ service.ServiceProvider = SystemdService{}

func (s SystemdService) opts() systemctl.Options {
	return systemctl.Options{UserMode: s.userMode}
}

func (s SystemdService) Properties() (map[string]interface{}, error) {
	return s.props, nil
}

func init() {
	service.RegisterProvider(SystemdService{})
}

func (s SystemdService) Disable(ctx context.Context) error {
	return sysDisable(ctx, s.name, s.opts())
}

func (s SystemdService) Enable(ctx context.Context) error {
	return sysEnable(ctx, s.name, s.opts())
}

func (s SystemdService) Parse(id, method string, properties map[string]interface{}) (service.ServiceProvider, error) {
	name, userMode := "", false
	if properties == nil {
		properties = make(map[string]interface{})
	}
	if properties["name"] == nil {
		return nil, ingredients.ErrMissingName
	}
	if nameI, ok := properties["name"].(string); !ok || nameI == "" {
		return nil, ingredients.ErrMissingName
	} else {
		name = nameI
	}
	if properties["userMode"] != nil {
		if umI, ok := properties["userMode"].(bool); ok {
			userMode = umI
		}
	}
	return SystemdService{id: id, name: name, method: method, userMode: userMode, props: properties}, nil
}

func (s SystemdService) Unmask(ctx context.Context) error {
	return sysUnmask(ctx, s.name, s.opts())
}

func (s SystemdService) Mask(ctx context.Context) error {
	return sysMask(ctx, s.name, s.opts())
}

func (s SystemdService) Restart(ctx context.Context) error {
	return sysRestart(ctx, s.name, s.opts())
}

func (s SystemdService) Reload(ctx context.Context) error {
	return sysReload(ctx, s.name, s.opts())
}

func (s SystemdService) Stop(ctx context.Context) error {
	return sysStop(ctx, s.name, s.opts())
}

func (s SystemdService) Start(ctx context.Context) error {
	return sysStart(ctx, s.name, s.opts())
}

func (s SystemdService) Status(ctx context.Context) (string, error) {
	return sysStatus(ctx, s.name, s.opts())
}

func (s SystemdService) InitName() string {
	return "systemd"
}

func (s SystemdService) IsInit() bool {
	if _, ok := os.Stat("/run/systemd/system"); ok == nil {
		return true
	}
	return false
}

func (s SystemdService) IsEnabled(ctx context.Context) (bool, error) {
	return sysIsEnabled(ctx, s.name, s.opts())
}

func (s SystemdService) IsMasked(ctx context.Context) (bool, error) {
	return sysIsMasked(ctx, s.name, s.opts())
}

func (s SystemdService) IsRunning(ctx context.Context) (bool, error) {
	return sysIsRunning(ctx, s.name, s.opts())
}
