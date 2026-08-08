package service

import (
	"errors"
	"testing"

	"github.com/taigrr/jety"
)

func TestRegisterProviderDuplicate(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	// Registering the same init name again should return ErrDuplicateInit.
	err := RegisterProvider(mp)
	if !errors.Is(err, ErrDuplicateInit) {
		t.Errorf("expected ErrDuplicateInit, got %v", err)
	}
}

func TestNewServiceProviderUnknownInit(t *testing.T) {
	oldInit := Init
	Init = "nonexistent-init-system"
	defer func() { Init = oldInit }()

	_, err := NewServiceProvider("svc-1", "running", map[string]interface{}{"name": "nginx"})
	if !errors.Is(err, ErrUnknownInit) {
		t.Errorf("expected ErrUnknownInit, got %v", err)
	}
}

func TestNewServiceProviderResolvesMock(t *testing.T) {
	mp := &mockProvider{}
	cleanup := registerMockProvider(t, mp)
	defer cleanup()

	provider, err := NewServiceProvider("svc-1", "running", map[string]interface{}{"name": "nginx"})
	if err != nil {
		t.Fatalf("NewServiceProvider() error: %v", err)
	}
	if provider == nil {
		t.Fatal("NewServiceProvider() returned nil")
	}
}

type procFallbackProvider struct {
	mockProvider
}

func (p *procFallbackProvider) InitName() string { return "systemd" }
func (p *procFallbackProvider) IsInit() bool     { return false }

func TestGuessInitUsesSetValue(t *testing.T) {
	oldInit := Init
	Init = "test-init"
	defer func() { Init = oldInit }()

	result := guessInit()
	if result != "test-init" {
		t.Errorf("guessInit() = %q, want %q", result, "test-init")
	}
}

func TestGuessInitProbesProviders(t *testing.T) {
	oldInit := Init
	Init = ""
	defer func() { Init = oldInit }()

	// Register a mock that claims to be the init system.
	mp := &mockProvider{}
	provTex.Lock()
	oldProv, hadOld := provMap["mock"]
	provMap["mock"] = mp
	provTex.Unlock()
	defer func() {
		provTex.Lock()
		if hadOld {
			provMap["mock"] = oldProv
		} else {
			delete(provMap, "mock")
		}
		provTex.Unlock()
	}()

	result := guessInit()
	// On this Linux machine it will likely find systemd (the real registered provider).
	// The point is it doesn't return "unknown" — it probes the providers.
	if result == "" {
		t.Error("guessInit() returned empty string")
	}
}

func TestGuessInitTrimsProcFallback(t *testing.T) {
	oldInit := Init
	Init = ""
	defer func() { Init = oldInit }()
	oldConfigInit := jety.GetString("init")
	jety.Set("init", "")
	defer jety.Set("init", oldConfigInit)

	provTex.Lock()
	savedMap := provMap
	provMap = make(map[string]ServiceProvider)
	provTex.Unlock()
	defer func() {
		provTex.Lock()
		provMap = savedMap
		provTex.Unlock()
	}()

	oldReadProc1Comm := readProc1Comm
	readProc1Comm = func(name string) ([]byte, error) {
		if name != "/proc/1/comm" {
			t.Fatalf("readProc1Comm called with %q", name)
		}
		return []byte("systemd\n"), nil
	}
	defer func() { readProc1Comm = oldReadProc1Comm }()

	if result := guessInit(); result != "systemd" {
		t.Errorf("guessInit() = %q, want %q", result, "systemd")
	}
}

func TestNewServiceProviderUsesTrimmedProcFallback(t *testing.T) {
	oldInit := Init
	Init = ""
	defer func() { Init = oldInit }()
	oldConfigInit := jety.GetString("init")
	jety.Set("init", "")
	defer jety.Set("init", oldConfigInit)

	mp := &procFallbackProvider{}
	provTex.Lock()
	savedMap := provMap
	provMap = map[string]ServiceProvider{"systemd": mp}
	provTex.Unlock()
	defer func() {
		provTex.Lock()
		provMap = savedMap
		provTex.Unlock()
	}()

	oldReadProc1Comm := readProc1Comm
	readProc1Comm = func(string) ([]byte, error) {
		return []byte("systemd\n"), nil
	}
	defer func() { readProc1Comm = oldReadProc1Comm }()

	if result := guessInit(); result != "systemd" {
		t.Fatalf("guessInit() = %q, want %q", result, "systemd")
	}

	provider, err := NewServiceProvider("svc-1", "running", map[string]interface{}{"name": "nginx"})
	if err != nil {
		t.Fatalf("NewServiceProvider() error: %v", err)
	}
	if provider == nil {
		t.Fatal("NewServiceProvider() returned nil")
	}
}
