//go:build freebsd || netbsd || openbsd || dragonfly

package rcd

import (
	"context"
	"os"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/internal/ingredients/service"
	"github.com/taigrr/rcd"
)

// RCDService implements service.ServiceProvider for BSD rc.d init systems,
// delegating to the github.com/taigrr/rcd library.
type RCDService struct {
	id     string
	name   string
	method string
	props  map[string]interface{}
	opts   rcd.Options
}

func init() {
	service.RegisterProvider(RCDService{})
}

func (s RCDService) Properties() (map[string]interface{}, error) {
	return s.props, nil
}

func (s RCDService) Parse(id, method string, properties map[string]interface{}) (service.ServiceProvider, error) {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	nameI, ok := properties["name"]
	if !ok {
		return nil, ingredients.ErrMissingName
	}
	name, ok := nameI.(string)
	if !ok || name == "" {
		return nil, ingredients.ErrMissingName
	}
	return RCDService{id: id, name: name, method: method, props: properties}, nil
}

func (s RCDService) InitName() string {
	return "rcd"
}

func (s RCDService) IsInit() bool {
	info, err := os.Stat("/etc/rc.d")
	return err == nil && info.IsDir()
}

func (s RCDService) Start(ctx context.Context) error {
	return rcd.Start(ctx, s.name, s.opts)
}

func (s RCDService) Stop(ctx context.Context) error {
	return rcd.Stop(ctx, s.name, s.opts)
}

func (s RCDService) Restart(ctx context.Context) error {
	return rcd.Restart(ctx, s.name, s.opts)
}

func (s RCDService) Reload(ctx context.Context) error {
	return rcd.Reload(ctx, s.name, s.opts)
}

func (s RCDService) Status(ctx context.Context) (string, error) {
	return rcd.Status(ctx, s.name, s.opts)
}

func (s RCDService) IsRunning(ctx context.Context) (bool, error) {
	return rcd.IsActive(ctx, s.name, s.opts)
}

func (s RCDService) Enable(ctx context.Context) error {
	return rcd.Enable(ctx, s.name, s.opts)
}

func (s RCDService) Disable(ctx context.Context) error {
	return rcd.Disable(ctx, s.name, s.opts)
}

func (s RCDService) IsEnabled(ctx context.Context) (bool, error) {
	return rcd.IsEnabled(ctx, s.name, s.opts)
}

func (s RCDService) Mask(ctx context.Context) error {
	return rcd.Mask(ctx, s.name, s.opts)
}

func (s RCDService) Unmask(ctx context.Context) error {
	return rcd.Unmask(ctx, s.name, s.opts)
}

func (s RCDService) IsMasked(ctx context.Context) (bool, error) {
	return rcd.IsMasked(ctx, s.name, s.opts)
}
