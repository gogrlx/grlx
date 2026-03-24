//go:build linux

package openrc

import (
	"context"
	"errors"
	"testing"

	"github.com/taigrr/openrc"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

// stubState tracks calls made to stubbed openrc functions.
type stubState struct {
	calls []string
}

func (ss *stubState) record(op string) { ss.calls = append(ss.calls, op) }

// installStubs replaces all orc* function variables with recording stubs that
// succeed by default. It restores the originals on test cleanup.
func installStubs(t *testing.T) *stubState {
	t.Helper()
	ss := &stubState{}

	origStart, origStop := orcStart, orcStop
	origRestart, origReload := orcRestart, orcReload
	origEnable, origDisable := orcEnable, orcDisable
	origStatus := orcStatus
	origIsActive := orcIsActive
	origIsEnabled := orcIsEnabled
	origIsOpenRC := orcIsOpenRC

	orcStart = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		ss.record("start")
		return nil
	}
	orcStop = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		ss.record("stop")
		return nil
	}
	orcRestart = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		ss.record("restart")
		return nil
	}
	orcReload = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		ss.record("reload")
		return nil
	}
	orcEnable = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		ss.record("enable")
		return nil
	}
	orcDisable = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		ss.record("disable")
		return nil
	}
	orcStatus = func(_ context.Context, _ string, _ openrc.Options, _ ...string) (string, error) {
		ss.record("status")
		return "started", nil
	}
	orcIsActive = func(_ context.Context, _ string, _ openrc.Options) (bool, error) {
		ss.record("is-active")
		return true, nil
	}
	orcIsEnabled = func(_ context.Context, _ string, _ openrc.Options) (bool, error) {
		ss.record("is-enabled")
		return true, nil
	}
	orcIsOpenRC = func() bool {
		ss.record("is-openrc")
		return true
	}

	t.Cleanup(func() {
		orcStart, orcStop = origStart, origStop
		orcRestart, orcReload = origRestart, origReload
		orcEnable, orcDisable = origEnable, origDisable
		orcStatus = origStatus
		orcIsActive = origIsActive
		orcIsEnabled = origIsEnabled
		orcIsOpenRC = origIsOpenRC
	})
	return ss
}

func newSvc(name string) OpenRCService {
	return OpenRCService{
		id: "test-id", name: name, method: "running",
		props: map[string]interface{}{"name": name},
	}
}

// --- Parse tests ---

func TestOpenRCInitName(t *testing.T) {
	s := OpenRCService{}
	if name := s.InitName(); name != "openrc" {
		t.Errorf("InitName() = %q, want %q", name, "openrc")
	}
}

func TestOpenRCParseMissingName(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestOpenRCParseNilProperties(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", nil)
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for nil properties, got %v", err)
	}
}

func TestOpenRCParseEmptyName(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": ""})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for empty name, got %v", err)
	}
}

func TestOpenRCParseNameNotString(t *testing.T) {
	s := OpenRCService{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": 42})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for non-string name, got %v", err)
	}
}

func TestOpenRCParseValid(t *testing.T) {
	s := OpenRCService{}
	provider, err := s.Parse("svc-1", "running", map[string]interface{}{
		"name": "nginx",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	orc, ok := provider.(OpenRCService)
	if !ok {
		t.Fatal("Parse() did not return OpenRCService")
	}
	if orc.id != "svc-1" {
		t.Errorf("id = %q, want %q", orc.id, "svc-1")
	}
	if orc.name != "nginx" {
		t.Errorf("name = %q, want %q", orc.name, "nginx")
	}
	if orc.method != "running" {
		t.Errorf("method = %q, want %q", orc.method, "running")
	}
}

func TestOpenRCParseWithRunlevel(t *testing.T) {
	s := OpenRCService{}
	provider, err := s.Parse("svc-1", "enabled", map[string]interface{}{
		"name":     "sshd",
		"runlevel": "default",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	orc := provider.(OpenRCService)
	if orc.opts.Runlevel != "default" {
		t.Errorf("opts.Runlevel = %q, want %q", orc.opts.Runlevel, "default")
	}
}

func TestOpenRCProperties(t *testing.T) {
	props := map[string]interface{}{"name": "nginx"}
	s := OpenRCService{props: props}
	got, err := s.Properties()
	if err != nil {
		t.Fatalf("Properties() error: %v", err)
	}
	if got["name"] != "nginx" {
		t.Errorf("Properties()[name] = %v, want %q", got["name"], "nginx")
	}
}

// --- Mask/Unmask/IsMasked ---

func TestOpenRCMaskNotSupported(t *testing.T) {
	s := OpenRCService{name: "nginx"}
	ctx := context.Background()

	if err := s.Mask(ctx); err != ErrMaskNotSupported {
		t.Errorf("Mask() = %v, want ErrMaskNotSupported", err)
	}
	if err := s.Unmask(ctx); err != ErrMaskNotSupported {
		t.Errorf("Unmask() = %v, want ErrMaskNotSupported", err)
	}
}

func TestOpenRCIsMaskedAlwaysFalse(t *testing.T) {
	s := OpenRCService{name: "nginx"}
	masked, err := s.IsMasked(context.Background())
	if err != nil {
		t.Fatalf("IsMasked() error: %v", err)
	}
	if masked {
		t.Error("IsMasked() should always return false for OpenRC")
	}
}

// --- IsInit ---

func TestOpenRCIsInit(t *testing.T) {
	ss := installStubs(t)
	s := OpenRCService{}
	result := s.IsInit()
	if !result {
		t.Error("IsInit() should return true with stubbed orcIsOpenRC")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "is-openrc" {
		t.Errorf("expected [is-openrc], got %v", ss.calls)
	}
}

func TestOpenRCIsInitFalse(t *testing.T) {
	installStubs(t)
	orcIsOpenRC = func() bool { return false }
	s := OpenRCService{}
	if s.IsInit() {
		t.Error("IsInit() should return false when orcIsOpenRC returns false")
	}
}

// --- Action methods: success paths ---

func TestStart(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "start" {
		t.Errorf("expected [start], got %v", ss.calls)
	}
}

func TestStop(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "stop" {
		t.Errorf("expected [stop], got %v", ss.calls)
	}
}

func TestRestart(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	if err := svc.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "restart" {
		t.Errorf("expected [restart], got %v", ss.calls)
	}
}

func TestReload(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	if err := svc.Reload(context.Background()); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "reload" {
		t.Errorf("expected [reload], got %v", ss.calls)
	}
}

func TestEnable(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	if err := svc.Enable(context.Background()); err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "enable" {
		t.Errorf("expected [enable], got %v", ss.calls)
	}
}

func TestDisable(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	if err := svc.Disable(context.Background()); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if len(ss.calls) != 1 || ss.calls[0] != "disable" {
		t.Errorf("expected [disable], got %v", ss.calls)
	}
}

func TestStatus(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status != "started" {
		t.Errorf("Status() = %q, want %q", status, "started")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "status" {
		t.Errorf("expected [status], got %v", ss.calls)
	}
}

func TestIsRunning(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
	running, err := svc.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("IsRunning() error: %v", err)
	}
	if !running {
		t.Error("IsRunning() = false, want true")
	}
	if len(ss.calls) != 1 || ss.calls[0] != "is-active" {
		t.Errorf("expected [is-active], got %v", ss.calls)
	}
}

func TestIsEnabled(t *testing.T) {
	ss := installStubs(t)
	svc := newSvc("nginx")
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

// --- Action methods: error paths ---

func TestStartError(t *testing.T) {
	installStubs(t)
	orcStart = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		return errors.New("start failed")
	}
	svc := newSvc("nginx")
	if err := svc.Start(context.Background()); err == nil || err.Error() != "start failed" {
		t.Errorf("Start() error = %v, want 'start failed'", err)
	}
}

func TestStopError(t *testing.T) {
	installStubs(t)
	orcStop = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		return errors.New("stop failed")
	}
	svc := newSvc("nginx")
	if err := svc.Stop(context.Background()); err == nil || err.Error() != "stop failed" {
		t.Errorf("Stop() error = %v, want 'stop failed'", err)
	}
}

func TestRestartError(t *testing.T) {
	installStubs(t)
	orcRestart = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		return errors.New("restart failed")
	}
	svc := newSvc("nginx")
	if err := svc.Restart(context.Background()); err == nil || err.Error() != "restart failed" {
		t.Errorf("Restart() error = %v, want 'restart failed'", err)
	}
}

func TestReloadError(t *testing.T) {
	installStubs(t)
	orcReload = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		return errors.New("reload failed")
	}
	svc := newSvc("nginx")
	if err := svc.Reload(context.Background()); err == nil || err.Error() != "reload failed" {
		t.Errorf("Reload() error = %v, want 'reload failed'", err)
	}
}

func TestEnableError(t *testing.T) {
	installStubs(t)
	orcEnable = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		return errors.New("enable failed")
	}
	svc := newSvc("nginx")
	if err := svc.Enable(context.Background()); err == nil || err.Error() != "enable failed" {
		t.Errorf("Enable() error = %v, want 'enable failed'", err)
	}
}

func TestDisableError(t *testing.T) {
	installStubs(t)
	orcDisable = func(_ context.Context, _ string, _ openrc.Options, _ ...string) error {
		return errors.New("disable failed")
	}
	svc := newSvc("nginx")
	if err := svc.Disable(context.Background()); err == nil || err.Error() != "disable failed" {
		t.Errorf("Disable() error = %v, want 'disable failed'", err)
	}
}

func TestStatusError(t *testing.T) {
	installStubs(t)
	orcStatus = func(_ context.Context, _ string, _ openrc.Options, _ ...string) (string, error) {
		return "", errors.New("status failed")
	}
	svc := newSvc("nginx")
	if _, err := svc.Status(context.Background()); err == nil || err.Error() != "status failed" {
		t.Errorf("Status() error = %v, want 'status failed'", err)
	}
}

func TestIsRunningError(t *testing.T) {
	installStubs(t)
	orcIsActive = func(_ context.Context, _ string, _ openrc.Options) (bool, error) {
		return false, errors.New("is-active failed")
	}
	svc := newSvc("nginx")
	if _, err := svc.IsRunning(context.Background()); err == nil || err.Error() != "is-active failed" {
		t.Errorf("IsRunning() error = %v, want 'is-active failed'", err)
	}
}

func TestIsEnabledError(t *testing.T) {
	installStubs(t)
	orcIsEnabled = func(_ context.Context, _ string, _ openrc.Options) (bool, error) {
		return false, errors.New("is-enabled failed")
	}
	svc := newSvc("nginx")
	if _, err := svc.IsEnabled(context.Background()); err == nil || err.Error() != "is-enabled failed" {
		t.Errorf("IsEnabled() error = %v, want 'is-enabled failed'", err)
	}
}

// --- Boolean alternate paths ---

func TestIsRunningFalse(t *testing.T) {
	installStubs(t)
	orcIsActive = func(_ context.Context, _ string, _ openrc.Options) (bool, error) {
		return false, nil
	}
	svc := newSvc("nginx")
	running, err := svc.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("IsRunning() error: %v", err)
	}
	if running {
		t.Error("IsRunning() = true, want false")
	}
}

func TestIsEnabledFalse(t *testing.T) {
	installStubs(t)
	orcIsEnabled = func(_ context.Context, _ string, _ openrc.Options) (bool, error) {
		return false, nil
	}
	svc := newSvc("nginx")
	enabled, err := svc.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("IsEnabled() error: %v", err)
	}
	if enabled {
		t.Error("IsEnabled() = true, want false")
	}
}

// --- Runlevel passthrough ---

func TestRunlevelPassedThrough(t *testing.T) {
	installStubs(t)
	var capturedOpts []openrc.Options

	orcEnable = func(_ context.Context, _ string, opts openrc.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	orcDisable = func(_ context.Context, _ string, opts openrc.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}
	orcStart = func(_ context.Context, _ string, opts openrc.Options, _ ...string) error {
		capturedOpts = append(capturedOpts, opts)
		return nil
	}

	svc := OpenRCService{
		id: "test-id", name: "sshd", method: "running",
		props: map[string]interface{}{"name": "sshd"},
		opts:  openrc.Options{Runlevel: "boot"},
	}
	ctx := context.Background()
	_ = svc.Enable(ctx)
	_ = svc.Disable(ctx)
	_ = svc.Start(ctx)

	if len(capturedOpts) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(capturedOpts))
	}
	for i, opts := range capturedOpts {
		if opts.Runlevel != "boot" {
			t.Errorf("call %d: Runlevel = %q, want %q", i, opts.Runlevel, "boot")
		}
	}
}

// --- Service name passthrough ---

func TestServiceNamePassedThrough(t *testing.T) {
	installStubs(t)
	var capturedNames []string

	orcStart = func(_ context.Context, name string, _ openrc.Options, _ ...string) error {
		capturedNames = append(capturedNames, name)
		return nil
	}
	orcStop = func(_ context.Context, name string, _ openrc.Options, _ ...string) error {
		capturedNames = append(capturedNames, name)
		return nil
	}
	orcRestart = func(_ context.Context, name string, _ openrc.Options, _ ...string) error {
		capturedNames = append(capturedNames, name)
		return nil
	}
	orcReload = func(_ context.Context, name string, _ openrc.Options, _ ...string) error {
		capturedNames = append(capturedNames, name)
		return nil
	}

	svc := newSvc("my-daemon")
	ctx := context.Background()
	_ = svc.Start(ctx)
	_ = svc.Stop(ctx)
	_ = svc.Restart(ctx)
	_ = svc.Reload(ctx)

	for i, name := range capturedNames {
		if name != "my-daemon" {
			t.Errorf("call %d: name = %q, want %q", i, name, "my-daemon")
		}
	}
}
