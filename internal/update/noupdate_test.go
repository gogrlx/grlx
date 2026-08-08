//go:build !self_update
// +build !self_update

package selfupdate

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNoUpdateCheckForUpdatesReturnsCurrentVersionAndDisabledError(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{
		CurrentVersion: "v1.2.3",
	})

	version, available, err := updater.CheckForUpdates(context.Background())
	if !errors.Is(err, ErrSelfUpdateDisabled) {
		t.Fatalf("CheckForUpdates error = %v, want ErrSelfUpdateDisabled", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("version = %q, want v1.2.3", version)
	}
	if available {
		t.Fatal("available = true, want false")
	}
}

func TestNoUpdatePerformUpdateReturnsDisabledError(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{})

	if err := updater.PerformUpdate(context.Background(), "v1.2.3"); !errors.Is(err, ErrSelfUpdateDisabled) {
		t.Fatalf("PerformUpdate error = %v, want ErrSelfUpdateDisabled", err)
	}
}

func TestNoUpdateStartUpdateCheckerInvokesCallbackOnce(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{
		CurrentVersion: "v1.2.3",
	})

	called := make(chan struct{}, 1)
	updater.StartUpdateChecker(context.Background(), func(version string, available bool, err error) {
		if !errors.Is(err, ErrSelfUpdateDisabled) {
			t.Errorf("callback error = %v, want ErrSelfUpdateDisabled", err)
		}
		if version != "v1.2.3" {
			t.Errorf("callback version = %q, want v1.2.3", version)
		}
		if available {
			t.Error("callback available = true, want false")
		}
		called <- struct{}{}
	})

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("StartUpdateChecker did not invoke callback")
	}
}
