//go:build freebsd || netbsd || openbsd || dragonfly

package rcd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
	"github.com/gogrlx/grlx/v2/internal/ingredients/service"
)

const rcConfPath = "/etc/rc.conf"

// RCDService implements service.ServiceProvider for BSD rc.d init systems.
type RCDService struct {
	id     string
	name   string
	method string
	props  map[string]interface{}
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
	// BSD systems use rc.d if /etc/rc.d exists and PID 1 is init/rc.
	if info, err := os.Stat("/etc/rc.d"); err == nil && info.IsDir() {
		return true
	}
	return false
}

// rcScript returns the path to the rc.d script for this service.
func (s RCDService) rcScript() string {
	// Check /usr/local/etc/rc.d first (ports/packages), then /etc/rc.d (base).
	local := filepath.Join("/usr/local/etc/rc.d", s.name)
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return filepath.Join("/etc/rc.d", s.name)
}

func (s RCDService) serviceCmd(ctx context.Context, action string) error {
	cmd := exec.CommandContext(ctx, "service", s.name, action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("service %s %s: %w: %s", s.name, action, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s RCDService) Start(ctx context.Context) error {
	return s.serviceCmd(ctx, "start")
}

func (s RCDService) Stop(ctx context.Context) error {
	return s.serviceCmd(ctx, "stop")
}

func (s RCDService) Restart(ctx context.Context) error {
	return s.serviceCmd(ctx, "restart")
}

func (s RCDService) Status(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "service", s.name, "status")
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (s RCDService) IsRunning(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "service", s.name, "status")
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means not running.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

// rcVarName returns the rc.conf variable name for enabling this service.
// e.g. "nginx" -> "nginx_enable".
func (s RCDService) rcVarName() string {
	return s.name + "_enable"
}

func (s RCDService) Enable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sysrc", s.rcVarName()+"=YES")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysrc enable %s: %w: %s", s.name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s RCDService) Disable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sysrc", s.rcVarName()+"=NO")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysrc disable %s: %w: %s", s.name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s RCDService) IsEnabled(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "sysrc", "-n", s.rcVarName())
	out, err := cmd.Output()
	if err != nil {
		// Variable not set means not enabled.
		return false, nil
	}
	val := strings.TrimSpace(string(out))
	return strings.EqualFold(val, "yes"), nil
}

// Mask prevents a service from being started by removing its rc.d script
// execute bit. BSD doesn't have a formal mask concept like systemd.
func (s RCDService) Mask(ctx context.Context) error {
	script := s.rcScript()
	return os.Chmod(script, 0o444)
}

// Unmask restores the execute bit on the service's rc.d script.
func (s RCDService) Unmask(ctx context.Context) error {
	script := s.rcScript()
	return os.Chmod(script, 0o755)
}

// IsMasked checks if the rc.d script lacks execute permission.
func (s RCDService) IsMasked(ctx context.Context) (bool, error) {
	script := s.rcScript()
	info, err := os.Stat(script)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("service script not found: %s", script)
		}
		return false, err
	}
	// Masked if the script is not executable.
	return info.Mode()&0o111 == 0, nil
}
