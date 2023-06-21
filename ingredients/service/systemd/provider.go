package systemd

import (
	"context"
	"os"

	"github.com/taigrr/systemctl"

	"github.com/gogrlx/grlx/ingredients/service"
	"github.com/gogrlx/grlx/types"
)

type SystemdService struct {
	id       string
	name     string
	method   string
	userMode bool
	props    map[string]interface{}
}

func (s SystemdService) Properties() (map[string]interface{}, error) {
	return s.props, nil
}

func init() {
	service.RegisterProvider(SystemdService{})
}

func (s SystemdService) Disable(ctx context.Context) error {
	return systemctl.Disable(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Enable(ctx context.Context) error {
	return systemctl.Enable(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Parse(id, method string, properties map[string]interface{}) (types.ServiceProvider, error) {
	name, userMode := "", false
	if properties == nil {
		properties = make(map[string]interface{})
	}
	if properties["name"] == nil {
		return nil, types.ErrMissingName
	}
	if nameI, ok := properties["name"].(string); !ok || nameI == "" {
		return nil, types.ErrMissingName
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
	return systemctl.Unmask(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Mask(ctx context.Context) error {
	return systemctl.Mask(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Restart(ctx context.Context) error {
	return systemctl.Restart(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Stop(ctx context.Context) error {
	return systemctl.Stop(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Start(ctx context.Context) error {
	return systemctl.Start(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) Status(ctx context.Context) (string, error) {
	return systemctl.Status(ctx, s.name, systemctl.Options{UserMode: s.userMode})
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
	return systemctl.IsEnabled(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) IsMasked(ctx context.Context) (bool, error) {
	return systemctl.IsMasked(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}

func (s SystemdService) IsRunning(ctx context.Context) (bool, error) {
	return systemctl.IsRunning(ctx, s.name, systemctl.Options{UserMode: s.userMode})
}
