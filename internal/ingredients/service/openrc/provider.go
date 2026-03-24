//go:build linux

package openrc

import (
	"context"
	"errors"

	"github.com/taigrr/openrc"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/internal/ingredients/service"
)

// ErrMaskNotSupported is returned when mask/unmask is attempted on OpenRC,
// which does not natively support service masking.
var ErrMaskNotSupported = errors.New("openrc does not support mask/unmask")

// Function variables for openrc operations — replaceable in tests.
var (
	orcStart     = openrc.Start
	orcStop      = openrc.Stop
	orcRestart   = openrc.Restart
	orcReload    = openrc.Reload
	orcStatus    = openrc.Status
	orcIsActive  = openrc.IsActive
	orcEnable    = openrc.Enable
	orcDisable   = openrc.Disable
	orcIsEnabled = openrc.IsEnabled
	orcIsOpenRC  = openrc.IsOpenRC
)

// OpenRCService implements service.ServiceProvider for OpenRC init systems
// (Alpine Linux, Gentoo, Artix, etc.), delegating to the
// github.com/taigrr/openrc library.
type OpenRCService struct {
	id     string
	name   string
	method string
	props  map[string]interface{}
	opts   openrc.Options
}

// Compile-time interface check.
var _ service.ServiceProvider = OpenRCService{}

func init() {
	service.RegisterProvider(OpenRCService{})
}

func (s OpenRCService) Properties() (map[string]interface{}, error) {
	return s.props, nil
}

func (s OpenRCService) Parse(id, method string, properties map[string]interface{}) (service.ServiceProvider, error) {
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
	var opts openrc.Options
	if rl, ok := properties["runlevel"].(string); ok && rl != "" {
		opts.Runlevel = rl
	}
	return OpenRCService{
		id:     id,
		name:   name,
		method: method,
		props:  properties,
		opts:   opts,
	}, nil
}

func (s OpenRCService) InitName() string {
	return "openrc"
}

func (s OpenRCService) IsInit() bool {
	return orcIsOpenRC()
}

func (s OpenRCService) Start(ctx context.Context) error {
	return orcStart(ctx, s.name, s.opts)
}

func (s OpenRCService) Stop(ctx context.Context) error {
	return orcStop(ctx, s.name, s.opts)
}

func (s OpenRCService) Restart(ctx context.Context) error {
	return orcRestart(ctx, s.name, s.opts)
}

func (s OpenRCService) Reload(ctx context.Context) error {
	return orcReload(ctx, s.name, s.opts)
}

func (s OpenRCService) Status(ctx context.Context) (string, error) {
	return orcStatus(ctx, s.name, s.opts)
}

func (s OpenRCService) IsRunning(ctx context.Context) (bool, error) {
	return orcIsActive(ctx, s.name, s.opts)
}

func (s OpenRCService) Enable(ctx context.Context) error {
	return orcEnable(ctx, s.name, s.opts)
}

func (s OpenRCService) Disable(ctx context.Context) error {
	return orcDisable(ctx, s.name, s.opts)
}

func (s OpenRCService) IsEnabled(ctx context.Context) (bool, error) {
	return orcIsEnabled(ctx, s.name, s.opts)
}

// Mask is not supported by OpenRC.
func (s OpenRCService) Mask(_ context.Context) error {
	return ErrMaskNotSupported
}

// Unmask is not supported by OpenRC.
func (s OpenRCService) Unmask(_ context.Context) error {
	return ErrMaskNotSupported
}

// IsMasked always returns false for OpenRC since masking is not supported.
func (s OpenRCService) IsMasked(_ context.Context) (bool, error) {
	return false, nil
}
