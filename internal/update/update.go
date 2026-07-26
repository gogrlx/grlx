//go:build self_update
// +build self_update

// Package selfupdate is an UNWIRED, INCOMPLETE skeleton for binary
// self-updates. It is gated behind the `self_update` build tag and is not
// referenced by any grlx binary (the default build uses the no-op variant in
// noupdate.go). Do NOT enable it as-is:
//
//   - CheckForUpdates returns a hardcoded "placeholder" version and therefore
//     always reports that an update is available.
//   - PerformUpdate/replaceBinary download and swap the running binary WITHOUT
//     verifying the release signature or checksum (a remote-code-execution
//     vector).
//
// Implementing this safely requires design decisions (authoritative version
// source, signature verification against the published checksums.txt(.sig), and
// rollout policy). Tracked in
// https://github.com/gogrlx/grlx/issues/286.

package selfupdate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// UpdateConfig holds the configuration for self-updates
type UpdateConfig struct {
	CurrentVersion string
	BinaryName     string
	UpdateURL      string
	CheckInterval  time.Duration
}

// Updater handles self-update functionality
type Updater struct {
	config UpdateConfig
	client *http.Client
}

// NewUpdater creates a new updater instance
func NewUpdater(config UpdateConfig) *Updater {
	return &Updater{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckForUpdates checks if a newer version is available
func (u *Updater) CheckForUpdates(ctx context.Context) (string, bool, error) {
	// Implementation would check against your release API
	// This is a skeleton - you'll need to implement based on your versioning strategy

	req, err := http.NewRequestWithContext(ctx, "GET", u.config.UpdateURL+"/latest", nil)
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("update check failed with status: %d", resp.StatusCode)
	}

	// Parse response to get latest version
	// This is a placeholder - implement based on your API response format
	latestVersion := "placeholder"

	if latestVersion != u.config.CurrentVersion {
		return latestVersion, true, nil
	}

	return u.config.CurrentVersion, false, nil
}

// PerformUpdate downloads and installs a new version
func (u *Updater) PerformUpdate(ctx context.Context, version string) error {
	downloadURL := fmt.Sprintf("%s/%s/%s-%s-%s-%s",
		u.config.UpdateURL, version, u.config.BinaryName, version, runtime.GOOS, runtime.GOARCH)

	// Download the new binary
	tempFile, err := u.downloadBinary(ctx, downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer os.Remove(tempFile)

	// Replace the current binary
	return u.replaceBinary(tempFile)
}

// downloadBinary downloads the binary to a temporary file
func (u *Updater) downloadBinary(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "grlx-update-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Copy the binary
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	// Make it executable
	err = os.Chmod(tempFile.Name(), 0o755)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// replaceBinary replaces the current binary with the new one
func (u *Updater) replaceBinary(newBinaryPath string) error {
	// Get the current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Resolve symlinks
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	return replaceBinaryAt(currentExe, newBinaryPath)
}

func replaceBinaryAt(currentExe, newBinaryPath string) error {
	stagedPath, err := stageBinary(currentExe, newBinaryPath)
	if err != nil {
		return err
	}
	defer os.Remove(stagedPath)

	return commitBinaryUpdate(currentExe, stagedPath)
}

func stageBinary(currentExe, newBinaryPath string) (string, error) {
	newBinary, err := os.Open(newBinaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open update binary: %w", err)
	}
	defer newBinary.Close()

	newBinaryInfo, err := newBinary.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to inspect update binary: %w", err)
	}
	if newBinaryInfo.IsDir() {
		return "", fmt.Errorf("update binary is a directory: %s", newBinaryPath)
	}

	targetDir := filepath.Dir(currentExe)
	stagedFile, err := os.CreateTemp(targetDir, ".grlx-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to stage update in executable directory: %w", err)
	}
	stagedPath := stagedFile.Name()
	removeStaged := true
	defer func() {
		if removeStaged {
			os.Remove(stagedPath)
		}
	}()

	if _, err := io.Copy(stagedFile, newBinary); err != nil {
		stagedFile.Close()
		return "", fmt.Errorf("failed to stage update binary: %w", err)
	}
	if err := stagedFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close staged update binary: %w", err)
	}
	if err := os.Chmod(stagedPath, newBinaryInfo.Mode().Perm()); err != nil {
		return "", fmt.Errorf("failed to make staged update executable: %w", err)
	}

	removeStaged = false
	return stagedPath, nil
}

func commitBinaryUpdate(currentExe, stagedPath string) error {
	// Create backup
	backupPath := currentExe + ".backup"
	err := os.Rename(currentExe, backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Move new binary into place
	err = os.Rename(stagedPath, currentExe)
	if err != nil {
		// Restore backup on failure
		if restoreErr := os.Rename(backupPath, currentExe); restoreErr != nil {
			return fmt.Errorf("failed to install update: %w; failed to restore backup: %w", err, restoreErr)
		}
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Remove backup on success
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to remove update backup: %w", err)
	}

	return nil
}

// StartUpdateChecker starts a background goroutine that periodically checks for updates
func (u *Updater) StartUpdateChecker(ctx context.Context, callback func(version string, available bool, err error)) {
	ticker := time.NewTicker(u.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			version, available, err := u.CheckForUpdates(ctx)
			if callback != nil {
				callback(version, available, err)
			}
		}
	}
}
