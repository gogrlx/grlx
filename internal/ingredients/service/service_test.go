package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestServiceMethods(t *testing.T) {
	s := Service{}
	name, methods := s.Methods()
	if name != "service" {
		t.Errorf("expected ingredient name 'service', got %q", name)
	}
	expected := []string{
		"disabled", "enabled", "masked", "reloaded",
		"restarted", "running", "stopped", "unmasked",
	}
	if len(methods) != len(expected) {
		t.Fatalf("expected %d methods, got %d", len(expected), len(methods))
	}
	for i, m := range methods {
		if m != expected[i] {
			t.Errorf("method[%d]: expected %q, got %q", i, expected[i], m)
		}
	}
}

func TestServiceParseMissingName(t *testing.T) {
	s := Service{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName, got %v", err)
	}
}

func TestServiceParseNilProperties(t *testing.T) {
	s := Service{}
	_, err := s.Parse("test-id", "running", nil)
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for nil properties, got %v", err)
	}
}

func TestServiceParseNameNotString(t *testing.T) {
	s := Service{}
	_, err := s.Parse("test-id", "running", map[string]interface{}{"name": 42})
	if err != ingredients.ErrMissingName {
		t.Errorf("expected ErrMissingName for non-string name, got %v", err)
	}
}

func TestServiceParseInvalidMethod(t *testing.T) {
	s := Service{}
	_, err := s.Parse("test-id", "nonexistent", map[string]interface{}{"name": "nginx"})
	if err != ingredients.ErrInvalidMethod {
		t.Errorf("expected ErrInvalidMethod, got %v", err)
	}
}

func TestServiceParseValidMethods(t *testing.T) {
	s := Service{}
	methods := []string{
		"disabled", "enabled", "masked", "reloaded",
		"restarted", "running", "stopped", "unmasked",
	}
	for _, method := range methods {
		cooker, err := s.Parse("test-id", method, map[string]interface{}{"name": "nginx"})
		if err != nil {
			t.Errorf("Parse(%q) returned error: %v", method, err)
			continue
		}
		parsed, ok := cooker.(Service)
		if !ok {
			t.Errorf("Parse(%q) returned wrong type", method)
			continue
		}
		if parsed.name != "nginx" {
			t.Errorf("Parse(%q) name = %q, want %q", method, parsed.name, "nginx")
		}
		if parsed.method != method {
			t.Errorf("Parse(%q) method = %q, want %q", method, parsed.method, method)
		}
		if parsed.id != "test-id" {
			t.Errorf("Parse(%q) id = %q, want %q", method, parsed.id, "test-id")
		}
	}
}

func TestServiceProperties(t *testing.T) {
	props := map[string]interface{}{"name": "nginx", "userMode": true}
	s := Service{properties: props}
	got, err := s.Properties()
	if err != nil {
		t.Fatalf("Properties() returned error: %v", err)
	}
	if got["name"] != "nginx" {
		t.Errorf("Properties()[name] = %v, want %q", got["name"], "nginx")
	}
}

func TestServicePropertiesForMethod(t *testing.T) {
	s := Service{}
	props, err := s.PropertiesForMethod("running")
	if err != nil {
		t.Errorf("PropertiesForMethod() returned error: %v", err)
	}
	if props != nil {
		t.Errorf("PropertiesForMethod() = %v, want nil", props)
	}
}

// mockProvider implements ServiceProvider for testing Apply/Test methods.
type mockProvider struct {
	running bool
	enabled bool
	masked  bool

	startErr     error
	stopErr      error
	enableErr    error
	disableErr   error
	maskErr      error
	unmaskErr    error
	restartErr   error
	reloadErr    error
	isRunningErr error
	isEnabledErr error
	isMaskedErr  error
}

func (m *mockProvider) Properties() (map[string]interface{}, error) {
	return map[string]interface{}{"name": "mock-svc"}, nil
}

func (m *mockProvider) Parse(id, method string, properties map[string]interface{}) (ServiceProvider, error) {
	return m, nil
}

func (m *mockProvider) Start(_ context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.running = true
	return nil
}

func (m *mockProvider) Stop(_ context.Context) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.running = false
	return nil
}

func (m *mockProvider) Status(_ context.Context) (string, error) {
	if m.running {
		return "running", nil
	}
	return "stopped", nil
}

func (m *mockProvider) Enable(_ context.Context) error {
	if m.enableErr != nil {
		return m.enableErr
	}
	m.enabled = true
	return nil
}

func (m *mockProvider) Disable(_ context.Context) error {
	if m.disableErr != nil {
		return m.disableErr
	}
	m.enabled = false
	return nil
}

func (m *mockProvider) IsEnabled(_ context.Context) (bool, error) {
	return m.enabled, m.isEnabledErr
}
func (m *mockProvider) IsRunning(_ context.Context) (bool, error) {
	return m.running, m.isRunningErr
}

func (m *mockProvider) Restart(_ context.Context) error {
	if m.restartErr != nil {
		return m.restartErr
	}
	m.running = true
	return nil
}

func (m *mockProvider) Reload(_ context.Context) error {
	if m.reloadErr != nil {
		return m.reloadErr
	}
	return nil
}

func (m *mockProvider) Mask(_ context.Context) error {
	if m.maskErr != nil {
		return m.maskErr
	}
	m.masked = true
	return nil
}

func (m *mockProvider) Unmask(_ context.Context) error {
	if m.unmaskErr != nil {
		return m.unmaskErr
	}
	m.masked = false
	return nil
}

func (m *mockProvider) IsMasked(_ context.Context) (bool, error) {
	return m.masked, m.isMaskedErr
}
func (m *mockProvider) InitName() string { return "mock" }
func (m *mockProvider) IsInit() bool     { return true }

// registerMockProvider registers the mock and sets Init to "mock" so
// NewServiceProvider resolves to it. It returns a cleanup function.
func registerMockProvider(t *testing.T, mp *mockProvider) func() {
	t.Helper()
	oldInit := Init
	Init = "mock"
	provTex.Lock()
	oldProv, hadOld := provMap["mock"]
	provMap["mock"] = mp
	provTex.Unlock()
	return func() {
		Init = oldInit
		provTex.Lock()
		if hadOld {
			provMap["mock"] = oldProv
		} else {
			delete(provMap, "mock")
		}
		provTex.Unlock()
	}
}

func TestApplyRunning(t *testing.T) {
	mp := &mockProvider{running: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "running", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	// Should start the service (not running -> running).
	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(running) error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Error("Apply(running) should succeed")
	}
	if !result.Changed {
		t.Error("Apply(running) should report changed")
	}
	if !mp.running {
		t.Error("mock service should be running after Apply")
	}

	// Apply again — already running, no change.
	result, err = s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(running) second call error: %v", err)
	}
	if result.Changed {
		t.Error("Apply(running) should not report changed when already running")
	}
}

func TestApplyStopped(t *testing.T) {
	mp := &mockProvider{running: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "stopped", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(stopped) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(stopped) should report changed")
	}
	if mp.running {
		t.Error("mock service should be stopped after Apply")
	}

	// Already stopped.
	result, err = s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(stopped) second call error: %v", err)
	}
	if result.Changed {
		t.Error("Apply(stopped) should not report changed when already stopped")
	}
}

func TestApplyEnabled(t *testing.T) {
	mp := &mockProvider{enabled: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "enabled", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(enabled) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(enabled) should report changed")
	}

	result, err = s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(enabled) second call error: %v", err)
	}
	if result.Changed {
		t.Error("Apply(enabled) should not report changed when already enabled")
	}
}

func TestApplyDisabled(t *testing.T) {
	mp := &mockProvider{enabled: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "disabled", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(disabled) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(disabled) should report changed")
	}

	result, err = s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(disabled) second call error: %v", err)
	}
	if result.Changed {
		t.Error("Apply(disabled) should not report changed when already disabled")
	}
}

func TestApplyMasked(t *testing.T) {
	mp := &mockProvider{masked: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "masked", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(masked) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(masked) should report changed")
	}

	result, err = s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(masked) second call error: %v", err)
	}
	if result.Changed {
		t.Error("Apply(masked) should not report changed when already masked")
	}
}

func TestApplyUnmasked(t *testing.T) {
	mp := &mockProvider{masked: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "unmasked", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(unmasked) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(unmasked) should report changed")
	}

	result, err = s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(unmasked) second call error: %v", err)
	}
	if result.Changed {
		t.Error("Apply(unmasked) should not report changed when already unmasked")
	}
}

func TestApplyRestarted(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "restarted", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(restarted) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(restarted) should always report changed")
	}
}

func TestApplyReloaded(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "reloaded", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	result, err := s.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply(reloaded) error: %v", err)
	}
	if !result.Changed {
		t.Error("Apply(reloaded) should always report changed")
	}
}

func TestApplyInvalidMethod(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "bogus", properties: map[string]interface{}{"name": "nginx"}}
	ctx := context.Background()

	_, err := s.Apply(ctx)
	if !errors.Is(err, ingredients.ErrInvalidMethod) {
		t.Errorf("Apply(bogus) expected ErrInvalidMethod, got %v", err)
	}
}

func TestApplyStartError(t *testing.T) {
	mp := &mockProvider{running: false, startErr: errors.New("start failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "running", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from start failure")
	}
	if result.Succeeded {
		t.Error("result should not be succeeded on start error")
	}
	if !result.Failed {
		t.Error("result should be failed on start error")
	}
}

func TestApplyStopError(t *testing.T) {
	mp := &mockProvider{running: true, stopErr: errors.New("stop failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "stopped", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from stop failure")
	}
	if !result.Failed {
		t.Error("result should be failed on stop error")
	}
}

func TestApplyRestartError(t *testing.T) {
	mp := &mockProvider{restartErr: errors.New("restart failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "restarted", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from restart failure")
	}
	if !result.Failed {
		t.Error("result should be failed on restart error")
	}
}

func TestApplyReloadError(t *testing.T) {
	mp := &mockProvider{reloadErr: errors.New("reload failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "reloaded", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from reload failure")
	}
	if !result.Failed {
		t.Error("result should be failed on reload error")
	}
}

// Test mode tests
func TestTestRunning(t *testing.T) {
	mp := &mockProvider{running: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "running", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(running) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(running) should report changed when not running")
	}
	// Should NOT have actually started.
	if mp.running {
		t.Error("Test should not mutate state")
	}
}

func TestTestRunningAlready(t *testing.T) {
	mp := &mockProvider{running: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "running", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(running) error: %v", err)
	}
	if result.Changed {
		t.Error("Test(running) should not report changed when already running")
	}
}

func TestTestStopped(t *testing.T) {
	mp := &mockProvider{running: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "stopped", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(stopped) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(stopped) should report changed when running")
	}
}

func TestTestEnabled(t *testing.T) {
	mp := &mockProvider{enabled: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "enabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(enabled) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(enabled) should report changed when not enabled")
	}
}

func TestTestDisabled(t *testing.T) {
	mp := &mockProvider{enabled: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "disabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(disabled) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(disabled) should report changed when enabled")
	}
}

func TestTestMasked(t *testing.T) {
	mp := &mockProvider{masked: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "masked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(masked) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(masked) should report changed when not masked")
	}
}

func TestTestUnmasked(t *testing.T) {
	mp := &mockProvider{masked: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "unmasked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(unmasked) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(unmasked) should report changed when masked")
	}
}

func TestTestRestarted(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "restarted", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(restarted) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(restarted) should always report changed")
	}
}

func TestTestReloaded(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "reloaded", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(reloaded) error: %v", err)
	}
	if !result.Changed {
		t.Error("Test(reloaded) should always report changed")
	}
}

func TestTestInvalidMethod(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "bogus", properties: map[string]interface{}{"name": "nginx"}}
	_, err := s.Test(context.Background())
	if !errors.Is(err, ingredients.ErrInvalidMethod) {
		t.Errorf("Test(bogus) expected ErrInvalidMethod, got %v", err)
	}
}

// Apply error paths for enable/disable/mask/unmask.

func TestApplyEnableError(t *testing.T) {
	mp := &mockProvider{enabled: false, enableErr: errors.New("enable failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "enabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from enable failure")
	}
	if !result.Failed {
		t.Error("result should be failed on enable error")
	}
}

func TestApplyDisableError(t *testing.T) {
	mp := &mockProvider{enabled: true, disableErr: errors.New("disable failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "disabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from disable failure")
	}
	if !result.Failed {
		t.Error("result should be failed on disable error")
	}
}

func TestApplyMaskError(t *testing.T) {
	mp := &mockProvider{masked: false, maskErr: errors.New("mask failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "masked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from mask failure")
	}
	if !result.Failed {
		t.Error("result should be failed on mask error")
	}
}

func TestApplyUnmaskError(t *testing.T) {
	mp := &mockProvider{masked: true, unmaskErr: errors.New("unmask failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "unmasked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from unmask failure")
	}
	if !result.Failed {
		t.Error("result should be failed on unmask error")
	}
}

// Apply error paths for IsRunning/IsEnabled/IsMasked query failures.

func TestApplyRunningQueryError(t *testing.T) {
	mp := &mockProvider{isRunningErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "running", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from IsRunning query failure")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestApplyStoppedQueryError(t *testing.T) {
	mp := &mockProvider{isRunningErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "stopped", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from IsRunning query failure")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestApplyEnabledQueryError(t *testing.T) {
	mp := &mockProvider{isEnabledErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "enabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from IsEnabled query failure")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestApplyDisabledQueryError(t *testing.T) {
	mp := &mockProvider{isEnabledErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "disabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from IsEnabled query failure")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestApplyMaskedQueryError(t *testing.T) {
	mp := &mockProvider{isMaskedErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "masked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from IsMasked query failure")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestApplyUnmaskedQueryError(t *testing.T) {
	mp := &mockProvider{isMaskedErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "unmasked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Apply(context.Background())
	if err == nil {
		t.Error("expected error from IsMasked query failure")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

// Test mode — "already in desired state" paths.

func TestTestStoppedAlready(t *testing.T) {
	mp := &mockProvider{running: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "stopped", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(stopped) error: %v", err)
	}
	if result.Changed {
		t.Error("Test(stopped) should not report changed when already stopped")
	}
}

func TestTestEnabledAlready(t *testing.T) {
	mp := &mockProvider{enabled: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "enabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(enabled) error: %v", err)
	}
	if result.Changed {
		t.Error("Test(enabled) should not report changed when already enabled")
	}
}

func TestTestDisabledAlready(t *testing.T) {
	mp := &mockProvider{enabled: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "disabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(disabled) error: %v", err)
	}
	if result.Changed {
		t.Error("Test(disabled) should not report changed when already disabled")
	}
}

func TestTestMaskedAlready(t *testing.T) {
	mp := &mockProvider{masked: true}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "masked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(masked) error: %v", err)
	}
	if result.Changed {
		t.Error("Test(masked) should not report changed when already masked")
	}
}

func TestTestUnmaskedAlready(t *testing.T) {
	mp := &mockProvider{masked: false}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "unmasked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err != nil {
		t.Fatalf("Test(unmasked) error: %v", err)
	}
	if result.Changed {
		t.Error("Test(unmasked) should not report changed when already unmasked")
	}
}

// Test mode — query error paths.

func TestTestRunningQueryError(t *testing.T) {
	mp := &mockProvider{isRunningErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "running", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err == nil {
		t.Error("expected error from IsRunning query failure in Test")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestTestStoppedQueryError(t *testing.T) {
	mp := &mockProvider{isRunningErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "stopped", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err == nil {
		t.Error("expected error from IsRunning query failure in Test")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestTestEnabledQueryError(t *testing.T) {
	mp := &mockProvider{isEnabledErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "enabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err == nil {
		t.Error("expected error from IsEnabled query failure in Test")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestTestDisabledQueryError(t *testing.T) {
	mp := &mockProvider{isEnabledErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "disabled", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err == nil {
		t.Error("expected error from IsEnabled query failure in Test")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestTestMaskedQueryError(t *testing.T) {
	mp := &mockProvider{isMaskedErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "masked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err == nil {
		t.Error("expected error from IsMasked query failure in Test")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

func TestTestUnmaskedQueryError(t *testing.T) {
	mp := &mockProvider{isMaskedErr: errors.New("query failed")}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	s := Service{id: "svc-1", name: "nginx", method: "unmasked", properties: map[string]interface{}{"name": "nginx"}}
	result, err := s.Test(context.Background())
	if err == nil {
		t.Error("expected error from IsMasked query failure in Test")
	}
	if !result.Failed {
		t.Error("result should be failed on query error")
	}
}

// Providers: guessInit with config fallback.

func TestGuessInitFromConfig(t *testing.T) {
	oldInit := Init
	Init = ""
	defer func() { Init = oldInit }()

	// Clear all registered providers temporarily to prevent probing.
	provTex.Lock()
	savedMap := provMap
	provMap = make(map[string]ServiceProvider)
	provTex.Unlock()
	defer func() {
		provTex.Lock()
		provMap = savedMap
		provTex.Unlock()
	}()

	// config.Init() reads from jety — test is verifying the guessInit flow.
	// With no providers and Init="", it falls through to /proc/1/comm or "unknown".
	result := guessInit()
	if result == "" {
		t.Error("guessInit() should not return empty string")
	}
}

// NewServiceProvider with provider parse error propagation.

func TestNewServiceProviderParseError(t *testing.T) {
	// Register a provider whose Parse returns an error.
	errProv := &errParseProvider{}
	oldInit := Init
	Init = "errparse"
	provTex.Lock()
	provMap["errparse"] = errProv
	provTex.Unlock()
	defer func() {
		Init = oldInit
		provTex.Lock()
		delete(provMap, "errparse")
		provTex.Unlock()
	}()

	_, err := NewServiceProvider("svc-1", "running", map[string]interface{}{"name": "nginx"})
	if err == nil {
		t.Error("expected error from provider Parse failure")
	}
}

type errParseProvider struct{}

func (e *errParseProvider) Properties() (map[string]interface{}, error) { return nil, nil }
func (e *errParseProvider) Parse(_, _ string, _ map[string]interface{}) (ServiceProvider, error) {
	return nil, errors.New("parse failed")
}
func (e *errParseProvider) Start(_ context.Context) error             { return nil }
func (e *errParseProvider) Stop(_ context.Context) error              { return nil }
func (e *errParseProvider) Status(_ context.Context) (string, error)  { return "", nil }
func (e *errParseProvider) Enable(_ context.Context) error            { return nil }
func (e *errParseProvider) Disable(_ context.Context) error           { return nil }
func (e *errParseProvider) IsEnabled(_ context.Context) (bool, error) { return false, nil }
func (e *errParseProvider) IsRunning(_ context.Context) (bool, error) { return false, nil }
func (e *errParseProvider) Restart(_ context.Context) error           { return nil }
func (e *errParseProvider) Reload(_ context.Context) error            { return nil }
func (e *errParseProvider) Mask(_ context.Context) error              { return nil }
func (e *errParseProvider) Unmask(_ context.Context) error            { return nil }
func (e *errParseProvider) IsMasked(_ context.Context) (bool, error)  { return false, nil }
func (e *errParseProvider) InitName() string                          { return "errparse" }
func (e *errParseProvider) IsInit() bool                              { return false }
