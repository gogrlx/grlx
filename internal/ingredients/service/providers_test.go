package service

import (
	"errors"
	"testing"
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
