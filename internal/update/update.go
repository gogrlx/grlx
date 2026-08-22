//go:build self_update
// +build self_update

// Package selfupdate is an UNWIRED, INCOMPLETE skeleton for binary
// self-updates. It is gated behind the `self_update` build tag and is not
// referenced by any grlx binary (the default build uses the no-op variant in
// noupdate.go). Do NOT enable it as-is:
//
//   - PerformUpdate downloads and swaps the running binary WITHOUT verifying
//     the release signature or checksum (a remote-code-execution vector), so it
//     currently fails closed.
//
// Implementing this safely requires design decisions (authoritative version
// source, signature verification against the published checksums.txt(.sig), and
// rollout policy). Tracked in
// https://github.com/gogrlx/grlx/issues/286.

package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

var (
	errUnsignedUpdatesDisabled = errors.New("self-update install is disabled until release signature verification is implemented")
	errUpdateIntervalRequired  = errors.New("self-update check interval must be greater than zero")
)

const maxLatestReleaseResponseBytes = 1 << 20

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
	latestVersion, err := u.fetchLatestVersion(ctx)
	if err != nil {
		return "", false, err
	}

	updateAvailable, err := newerVersion(latestVersion, u.config.CurrentVersion)
	if err != nil {
		return "", false, err
	}
	if updateAvailable {
		return latestVersion, true, nil
	}

	return u.config.CurrentVersion, false, nil
}

func (u *Updater) fetchLatestVersion(ctx context.Context) (string, error) {
	if strings.TrimSpace(u.config.UpdateURL) == "" {
		return "", errors.New("update URL is required")
	}

	latestURL := strings.TrimRight(u.config.UpdateURL, "/") + "/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update check failed with status: %d", resp.StatusCode)
	}

	latestVersion, err := parseLatestVersion(resp.Body)
	if err != nil {
		return "", err
	}

	return latestVersion, nil
}

func parseLatestVersion(body io.Reader) (string, error) {
	contents, err := io.ReadAll(io.LimitReader(body, maxLatestReleaseResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read latest release response: %w", err)
	}
	if len(contents) > maxLatestReleaseResponseBytes {
		return "", fmt.Errorf("latest release response exceeds %d bytes", maxLatestReleaseResponseBytes)
	}

	var release struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Version    string `json:"version"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(contents, &release); err != nil {
		return "", fmt.Errorf("failed to parse latest release response: %w", err)
	}
	if release.Draft {
		return "", errors.New("latest release response is a draft")
	}
	if release.Prerelease {
		return "", errors.New("latest release response is a prerelease")
	}

	for _, candidate := range []string{release.TagName, release.Version, release.Name} {
		version := canonicalVersion(candidate)
		if version != "" {
			return version, nil
		}
	}

	return "", errors.New("latest release response did not contain a semantic version")
}

func newerVersion(latestVersion, currentVersion string) (bool, error) {
	latest := canonicalVersion(latestVersion)
	if latest == "" {
		return false, fmt.Errorf("latest version %q is not semantic", latestVersion)
	}

	current := canonicalVersion(currentVersion)
	if current == "" {
		return false, fmt.Errorf("current version %q is not semantic", currentVersion)
	}

	return semver.Compare(latest, current) > 0, nil
}

func canonicalVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return semver.Canonical(version)
}

// PerformUpdate downloads and installs a new version
func (u *Updater) PerformUpdate(ctx context.Context, version string) error {
	return errUnsignedUpdatesDisabled
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
	backupPath, err := backupPathFor(currentExe)
	if err != nil {
		return err
	}

	if err := os.Rename(currentExe, backupPath); err != nil {
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

func backupPathFor(currentExe string) (string, error) {
	targetDir := filepath.Dir(currentExe)
	targetName := filepath.Base(currentExe)

	backupFile, err := os.CreateTemp(targetDir, targetName+".backup-*")
	if err != nil {
		return "", fmt.Errorf("failed to reserve update backup path: %w", err)
	}
	backupPath := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("failed to close update backup placeholder: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return "", fmt.Errorf("failed to clear update backup placeholder: %w", err)
	}

	return backupPath, nil
}

// StartUpdateChecker starts a background goroutine that periodically checks for updates
func (u *Updater) StartUpdateChecker(ctx context.Context, callback func(version string, available bool, err error)) {
	if u.config.CheckInterval <= 0 {
		if callback != nil {
			callback("", false, errUpdateIntervalRequired)
		}
		return
	}

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
