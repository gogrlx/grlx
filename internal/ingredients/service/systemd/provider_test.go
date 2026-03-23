//go:build linux

package systemd

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/taigrr/systemctl"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

// stubState tracks calls made to stubbed systemctl functions.
type stubState struct {
	calls []string
}

func (ss *stubState) record(op string) { ss.calls = append(ss.calls, op) }

// installStubs replaces all sys* function variables with recording stubs that
// succeed by default. It restores the originals on test cleanup.
func installStubs(t *testing.T) *stubState {
	t.Helper()
	ss := &stubState{}

	origStart, origStop := sysStart, sysStop
	origRestart, origReload := sysRestart, sysReload
	origEnable, origDisable := sysEnable, sysDisable
	origMask, origUnmask := sysMask, sysUnmask
	origStatus := sysStatus
	origIsEnabled := sysIsEnabled
	origIsMasked, origIsRunning := sysIsMasked, sysIsRunning

	// Variadic-args stubs (Start, Stop, Restart, Reload, Enable, Disable, Mask, Unmask).
	sysStart = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("start")
		return nil
	}
	sysStop = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("stop")
		return nil
	}
	sysRestart = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("restart")
		return nil
	}
	sysReload = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("reload")
		return nil
	}
	sysEnable = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("enable")
		return nil
	}
	sysDisable = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("disable")
		return nil
	}
	sysMask = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("mask")
		return nil
	}
	sysUnmask = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		ss.record("unmask")
		return nil
	}
	// Status has variadic args.
	sysStatus = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) (string, error) {
		ss.record("status")
		return "active", nil
	}
	// IsEnabled has variadic args.
	sysIsEnabled = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) (bool, error) {
		ss.record("is-enabled")
		return true, nil
	}
	// IsMasked and IsRunning do NOT have variadic args.
	sysIsMasked = func(_ context.Context, _ string, _ systemctl.Options) (bool, error) {
		ss.record("is-masked")
		return false, nil
	}
	sysIsRunning = func(_ context.Context, _ string, _ systemctl.Options) (bool, error) {
		ss.record("is-running")
		return true, nil
	}

	t.Cleanup(func() {
		sysStart, sysStop = origStart, origStop
		sysRestart, sysReload = origRestart, origReload
		sysEnable, sysDisable = origEnable, origDisable
		sysMask, sysUnmask = origMask, origUnmask
		sysStatus = origStatus
		sysIsEnabled = origIsEnabled
		sysIsMasked, sysIsRunning = origIsMasked, origIsRunning
	})
	return ss
}

func newSvc(name string, userMode bool) SystemdService {
	return SystemdService{
		id: "test-id", name: name, method: "running",
		userMode: userMode, props: map[string]interface{}{"name": name},
	}
}

// --- Parse tests ---

func TestSystemdInitName(t *testing.T) {
	s := SystemdService{}
	if name := s.InitName(); name != "systemd" {
		t.Errorf("InitName() = %q, want %q", name, "systemd")
	}
}

func TestSystemdParseMissingName(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestSystemdParseNilProperties(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", nil)
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for nil properties, got %v", err)
	}
}

func TestSystemdParseEmptyName(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": ""})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for empty name, got %v", err)
	}
}

func TestSystemdParseNameNotString(t *testing.T) {
	s := SystemdService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": 123})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for non-string name, got %v", err)
	}
}

func TestSystemdParseValid(t *testing.T) {
	s := SystemdService{}
	provider, err := s.Parse("svc-1", "running", map[string]interface{}{"name": "nginx"})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	sd, ok := provider.(SystemdService)
	if !ok {
		t.Fatal("Parse() did not return SystemdService")
	}
	if sd.id != "svc-1" {
		t.Errorf("id = %q, want %q", sd.id, "svc-1")
	}
	if sd.name != "nginx" {
		t.Errorf("name = %q, want %q", sd.name, "nginx")
	}
	if sd.method != "running" {
		t.Errorf("method = %q, want %q", sd.method, "running")
	}
	if sd.userMode {
		t.Error("userMode should default to false")
	}
}

func TestSystemdParseUserMode(t *testing.T) {
	s := SystemdService{}
	provider, err := s.Parse("svc-1", "enabled", map[string]interface{}{
		"name": "podman", "userMode": true,
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	sd := provider.(SystemdService)
	if !sd.userMode {
		t.Error("userMode should be true when set in properties")
	}
}

func TestSystemdParseUserModeNonBool(t *testing.T) {
	s := SystemdService{}
	provider, err := s.Parse("svc-1", "enabled", map[string]interface{}{
		"name": "podman", "userMode": "yes",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	sd := provider.(SystemdService)
	if sd.userMode {
		t.Error("userMode should be false when set to non-bool value")
	}
}

func TestSystemdProperties(t *testing.T) {
	props := map[string]interface{}{"name": "nginx", "userMode": false}
	s := SystemdService{props: props}
	got, err := s.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if got["name"] != "nginx" {
		t.Errorf("Properties()[name] = %v, want %q", got["name"], "nginx")
	}
}

// --- IsInit ---

func TestIsInit(t *testing.T) {
	s := SystemdService{}
	result := s.IsInit()
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		if !result {
			t.Error("IsInit() should return true on a systemd system")
		}
	} else {
		if result {
			t.Error("IsInit() should return false on a non-systemd system")
		}
	}
}

// --- Action methods: success paths ---

func TestStart(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "start" {
		t.Errorf("expected [start], got %v", ss.calls)
	}
}

func TestStop(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "stop" {
		t.Errorf("expected [stop], got %v", ss.calls)
	}
}

func TestRestart(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "restart" {
		t.Errorf("expected [restart], got %v", ss.calls)
	}
}

func TestReload(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Reload(context.Background()); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "reload" {
		t.Errorf("expected [reload], got %v", ss.calls)
	}
}

func TestEnable(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Enable(context.Background()); err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "enable" {
		t.Errorf("expected [enable], got %v", ss.calls)
	}
}

func TestDisable(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Disable(context.Background()); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "disable" {
		t.Errorf("expected [disable], got %v", ss.calls)
	}
}

func TestMask(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Mask(context.Background()); err != nil {
		t.Fatalf("Mask() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "mask" {
		t.Errorf("expected [mask], got %v", ss.calls)
	}
}

func TestUnmask(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	if err := svc.Unmask(context.Background()); err != nil {
		t.Fatalf("Unmask() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "unmask" {
		t.Errorf("expected [unmask], got %v", ss.calls)
	}
}

func TestStatus(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status != "active" {
		t.Errorf("Status() = %q, want %q", status, "active")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "status" {
		t.Errorf("expected [status], got %v", ss.calls)
	}
}

func TestIsEnabled(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	enabled, err := svc.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("IsEnabled() error: %v", err)
	}
	if !enabled {
		t.Error("IsEnabled() = false, want true")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "is-enabled" {
		t.Errorf("expected [is-enabled], got %v", ss.calls)
	}
}

func TestIsMasked(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	masked, err := svc.IsMasked(context.Background())
	if err != nil {
		t.Fatalf("IsMasked() error: %v", err)
	}
	if masked {
		t.Error("IsMasked() = true, want false")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "is-masked" {
		t.Errorf("expected [is-masked], got %v", ss.calls)
	}
}

func TestIsRunning(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx", false)
	running, err := svc.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("IsRunning() error: %v", err)
	}
	if !running {
		t.Error("IsRunning() = false, want true")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "is-running" {
		t.Errorf("expected [is-running], got %v", ss.calls)
	}
}

// --- Action methods: error paths ---

func TestStartError(t *testing.T) {
	installStubs(t)
	sysStart = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("start failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Start(context.Background()); err == nil || err.Error() != "start failed" {
		t.Errorf("Start() error = %v, want 'start failed'", err)
	}
}

func TestStopError(t *testing.T) {
	installStubs(t)
	sysStop = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("stop failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Stop(context.Background()); err == nil || err.Error() != "stop failed" {
		t.Errorf("Stop() error = %v, want 'stop failed'", err)
	}
}

func TestRestartError(t *testing.T) {
	installStubs(t)
	sysRestart = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("restart failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Restart(context.Background()); err == nil || err.Error() != "restart failed" {
		t.Errorf("Restart() error = %v, want 'restart failed'", err)
	}
}

func TestReloadError(t *testing.T) {
	installStubs(t)
	sysReload = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("reload failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Reload(context.Background()); err == nil || err.Error() != "reload failed" {
		t.Errorf("Reload() error = %v, want 'reload failed'", err)
	}
}

func TestEnableError(t *testing.T) {
	installStubs(t)
	sysEnable = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("enable failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Enable(context.Background()); err == nil || err.Error() != "enable failed" {
		t.Errorf("Enable() error = %v, want 'enable failed'", err)
	}
}

func TestDisableError(t *testing.T) {
	installStubs(t)
	sysDisable = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("disable failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Disable(context.Background()); err == nil || err.Error() != "disable failed" {
		t.Errorf("Disable() error = %v, want 'disable failed'", err)
	}
}

func TestMaskError(t *testing.T) {
	installStubs(t)
	sysMask = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("mask failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Mask(context.Background()); err == nil || err.Error() != "mask failed" {
		t.Errorf("Mask() error = %v, want 'mask failed'", err)
	}
}

func TestUnmaskError(t *testing.T) {
	installStubs(t)
	sysUnmask = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) error {
		return errors.New("unmask failed")
	}
	svc := newSvc("nginx", false)
	if err := svc.Unmask(context.Background()); err == nil || err.Error() != "unmask failed" {
		t.Errorf("Unmask() error = %v, want 'unmask failed'", err)
	}
}

func TestStatusError(t *testing.T) {
	installStubs(t)
	sysStatus = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) (string, error) {
		return "", errors.New("status failed")
	}
	svc := newSvc("nginx", false)
	if _, err := svc.Status(context.Background()); err == nil || err.Error() != "status failed" {
		t.Errorf("Status() error = %v, want 'status failed'", err)
	}
}

func TestIsEnabledError(t *testing.T) {
	installStubs(t)
	sysIsEnabled = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) (bool, error) {
		return false, errors.New("is-enabled failed")
	}
	svc := newSvc("nginx", false)
	if _, err := svc.IsEnabled(context.Background()); err == nil || err.Error() != "is-enabled failed" {
		t.Errorf("IsEnabled() error = %v, want 'is-enabled failed'", err)
	}
}

func TestIsMaskedError(t *testing.T) {
	installStubs(t)
	sysIsMasked = func(_ context.Context, _ string, _ systemctl.Options) (bool, error) {
		return false, errors.New("is-masked failed")
	}
	svc := newSvc("nginx", false)
	if _, err := svc.IsMasked(context.Background()); err == nil || err.Error() != "is-masked failed" {
		t.Errorf("IsMasked() error = %v, want 'is-masked failed'", err)
	}
}

func TestIsRunningError(t *testing.T) {
	installStubs(t)
	sysIsRunning = func(_ context.Context, _ string, _ systemctl.Options) (bool, error) {
		return false, errors.New("is-running failed")
	}
	svc := newSvc("nginx", false)
	if _, err := svc.IsRunning(context.Background()); err == nil || err.Error() != "is-running failed" {
		t.Errorf("IsRunning() error = %v, want 'is-running failed'", err)
	}
}

// --- Boolean alternate paths ---

func TestIsEnabledFalse(t *testing.T) {
	installStubs(t)
	sysIsEnabled = func(_ context.Context, _ string, _ systemctl.Options, _ ...string) (bool, error) {
		return false, nil
	}
	svc := newSvc("nginx", false)
	enabled, err := svc.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("IsEnabled() error: %v", err)
	}
	if enabled {
		t.Error("IsEnabled() = true, want false")
	}
}

func TestIsMaskedTrue(t *testing.T) {
	installStubs(t)
	sysIsMasked = func(_ context.Context, _ string, _ systemctl.Options) (bool, error) {
		return true, nil
	}
	svc := newSvc("nginx", false)
	masked, err := svc.IsMasked(context.Background())
	if err != nil {
		t.Fatalf("IsMasked() error: %v", err)
	}
	if !masked {
		t.Error("IsMasked() = false, want true")
	}
}

func TestIsRunningFalse(t *testing.T) {
	installStubs(t)
	sysIsRunning = func(_ context.Context, _ string, _ systemctl.Options) (bool, error) {
		return false, nil
	}
	svc := newSvc("nginx", false)
	running, err := svc.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("IsRunning() error: %v", err)
	}
	if running {
		t.Error("IsRunning() = true, want false")
	}
}

// --- UserMode passthrough ---

func TestUserModePassedThrough(t *testing.T) {
	installStubs(t)
	var capturedOpts []systemctl.Options

	sysStart = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysStop = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysEnable = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysDisable = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysMask = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysUnmask = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysRestart = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysReload = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	sysStatus = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) (string, error) {
		capturedOpts = append(capturedOpts, opts)
		return "", nil
	}
	sysIsEnabled = func(_ context.Context, _ string, opts systemctl.Options, _ ...string) (bool, error) {
		capturedOpts = append(capturedOpts, opts)
		return false, nil
	}
	sysIsMasked = func(_ context.Context, _ string, opts systemctl.Options) (bool, error) {
		capturedOpts = append(capturedOpts, opts)
		return false, nil
	}
	sysIsRunning = func(_ context.Context, _ string, opts systemctl.Options) (bool, error) {
		capturedOpts = append(capturedOpts, opts)
		return false, nil
	}

	svc := newSvc("user-svc", true)
	ctx := context.Background()

	_ = svc.Start(ctx)
	_ = svc.Stop(ctx)
	_ = svc.Enable(ctx)
	_ = svc.Disable(ctx)
	_ = svc.Mask(ctx)
	_ = svc.Unmask(ctx)
	_ = svc.Restart(ctx)
	_ = svc.Reload(ctx)
	_, _ = svc.Status(ctx)
	_, _ = svc.IsEnabled(ctx)
	_, _ = svc.IsMasked(ctx)
	_, _ = svc.IsRunning(ctx)

	if len(capturedOpts) != 12 {
		t.Fatalf("expected 12 calls, got %d", len(capturedOpts))
	}
	for i, opts := range capturedOpts {
		if !opts.UserMode {
			t.Errorf("call %d: UserMode = false, want true", i)
		}
	}
}

// --- Unit name passthrough ---

func TestUnitNamePassedThrough(t *testing.T) {
	installStubs(t)
	var capturedUnits []string

	sysStart = func(_ context.Context, unit string, _ systemctl.Options, _ ...string) error {
		capturedUnits = append(capturedUnits, unit)
		return nil
	}
	sysStop = func(_ context.Context, unit string, _ systemctl.Options, _ ...string) error {
		capturedUnits = append(capturedUnits, unit)
		return nil
	}
	sysRestart = func(_ context.Context, unit string, _ systemctl.Options, _ ...string) error {
		capturedUnits = append(capturedUnits, unit)
		return nil
	}
	sysReload = func(_ context.Context, unit string, _ systemctl.Options, _ ...string) error {
		capturedUnits = append(capturedUnits, unit)
		return nil
	}

	svc := newSvc("my-daemon.service", false)
	ctx := context.Background()
	_ = svc.Start(ctx)
	_ = svc.Stop(ctx)
	_ = svc.Restart(ctx)
	_ = svc.Reload(ctx)

	for i, unit := range capturedUnits {
		if unit != "my-daemon.service" {
			t.Errorf("call %d: unit = %q, want %q", i, unit, "my-daemon.service")
		}
	}
}
