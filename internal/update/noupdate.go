//go:build no_self_update
// +build no_self_update

package selfupdate

import (
	"context"
	"errors"
	"time"
)

// UpdateConfig holds the configuration for self-updates (no-op version)
type UpdateConfig struct {
	CurrentVersion string
	BinaryName     string
	UpdateURL      string
	CheckInterval  time.Duration
}

// Updater handles self-update functionality (no-op version)
type Updater struct {
	config UpdateConfig
}

// ErrSelfUpdateDisabled is returned when self-update is disabled at build time
var ErrSelfUpdateDisabled = errors.New("self-update is disabled in this build")

// NewUpdater creates a new updater instance (no-op version)
func NewUpdater(config UpdateConfig) *Updater {
	return &Updater{
		config: config,
	}
}

// CheckForUpdates always returns that no updates are available
func (u *Updater) CheckForUpdates(ctx context.Context) (string, bool, error) {
	return u.config.CurrentVersion, false, ErrSelfUpdateDisabled
}

// PerformUpdate always returns an error indicating self-update is disabled
func (u *Updater) PerformUpdate(ctx context.Context, version string) error {
	return ErrSelfUpdateDisabled
}

// StartUpdateChecker does nothing in the no-update version
func (u *Updater) StartUpdateChecker(ctx context.Context, callback func(version string, available bool, err error)) {
	// No-op: self-update is disabled
	if callback != nil {
		callback(u.config.CurrentVersion, false, ErrSelfUpdateDisabled)
	}
}
