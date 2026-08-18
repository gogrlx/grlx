//go:build self_update
// +build self_update

package selfupdate

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckForUpdatesUsesLatestReleaseSemver(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/latest" {
			t.Fatalf("request path = %q, want /latest", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	t.Cleanup(server.Close)

	updater := NewUpdater(UpdateConfig{
		CurrentVersion: "v1.2.2",
		UpdateURL:      server.URL + "/",
	})

	version, available, err := updater.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("version = %q, want v1.2.3", version)
	}
	if !available {
		t.Fatal("available = false, want true")
	}
}

func TestCheckForUpdatesDoesNotDowngrade(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	t.Cleanup(server.Close)

	updater := NewUpdater(UpdateConfig{
		CurrentVersion: "v1.2.4",
		UpdateURL:      server.URL,
	})

	version, available, err := updater.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if version != "v1.2.4" {
		t.Fatalf("version = %q, want v1.2.4", version)
	}
	if available {
		t.Fatal("available = true, want false")
	}
}

func TestParseLatestVersionAcceptsFallbackFields(t *testing.T) {
	t.Parallel()

	version, err := parseLatestVersion(strings.NewReader(`{"version":"1.2.3"}`))
	if err != nil {
		t.Fatalf("parseLatestVersion returned error: %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("version = %q, want v1.2.3", version)
	}
}

func TestNewerVersionRejectsNonSemver(t *testing.T) {
	t.Parallel()

	if _, err := newerVersion("placeholder", "v1.2.3"); err == nil {
		t.Fatal("newerVersion returned nil error for invalid latest version")
	}
	if _, err := newerVersion("v1.2.3", "dev"); err == nil {
		t.Fatal("newerVersion returned nil error for invalid current version")
	}
}

func TestPerformUpdateFailsClosedWithoutSignatureVerification(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{})
	err := updater.PerformUpdate(context.Background(), "v1.2.3")
	if err == nil {
		t.Fatal("PerformUpdate returned nil error")
	}
	if !strings.Contains(err.Error(), "signature verification") {
		t.Fatalf("PerformUpdate error = %q, want signature verification failure", err)
	}
}

func TestStartUpdateCheckerRejectsMissingInterval(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{})
	called := false

	updater.StartUpdateChecker(context.Background(), func(version string, available bool, err error) {
		called = true
		if version != "" {
			t.Fatalf("version = %q, want empty", version)
		}
		if available {
			t.Fatal("available = true, want false")
		}
		if !errors.Is(err, errUpdateIntervalRequired) {
			t.Fatalf("error = %v, want %v", err, errUpdateIntervalRequired)
		}
	})

	if !called {
		t.Fatal("callback was not called")
	}
}

func TestVerifyBinaryChecksumAcceptsMatchingEntry(t *testing.T) {
	t.Parallel()

	binaryPath := filepath.Join(t.TempDir(), "grlx")
	contents := []byte("verified binary")
	if err := os.WriteFile(binaryPath, contents, 0o755); err != nil {
		t.Fatal(err)
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
	checksums := strings.NewReader("abcd unrelated\n" + checksum + "  grlx\n")

	if err := verifyBinaryChecksum(binaryPath, checksums); err != nil {
		t.Fatalf("verifyBinaryChecksum returned error: %v", err)
	}
}

func TestVerifyBinaryChecksumRejectsMismatch(t *testing.T) {
	t.Parallel()

	binaryPath := filepath.Join(t.TempDir(), "grlx")
	if err := os.WriteFile(binaryPath, []byte("actual"), 0o755); err != nil {
		t.Fatal(err)
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte("different")))
	err := verifyBinaryChecksum(binaryPath, strings.NewReader(checksum+"  grlx\n"))
	if err == nil {
		t.Fatal("verifyBinaryChecksum returned nil error for checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("verifyBinaryChecksum error = %q, want checksum mismatch", err)
	}
}

func TestVerifyBinaryChecksumRejectsMissingEntry(t *testing.T) {
	t.Parallel()

	binaryPath := filepath.Join(t.TempDir(), "grlx")
	if err := os.WriteFile(binaryPath, []byte("actual"), 0o755); err != nil {
		t.Fatal(err)
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte("actual")))
	err := verifyBinaryChecksum(binaryPath, strings.NewReader(checksum+"  sprout\n"))
	if err == nil {
		t.Fatal("verifyBinaryChecksum returned nil error for missing checksum entry")
	}
	if !strings.Contains(err.Error(), "checksum entry not found") {
		t.Fatalf("verifyBinaryChecksum error = %q, want missing entry", err)
	}
}

func TestVerifyBinaryChecksumRejectsMalformedChecksum(t *testing.T) {
	t.Parallel()

	binaryPath := filepath.Join(t.TempDir(), "grlx")
	if err := os.WriteFile(binaryPath, []byte("actual"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := verifyBinaryChecksum(binaryPath, strings.NewReader("not-a-sha256  grlx\n"))
	if err == nil {
		t.Fatal("verifyBinaryChecksum returned nil error for malformed checksum")
	}
	if !strings.Contains(err.Error(), "malformed SHA256 checksum") {
		t.Fatalf("verifyBinaryChecksum error = %q, want malformed checksum", err)
	}
}

func TestStageBinaryUsesExecutableDirectory(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	currentExe := filepath.Join(tempDir, "grlx")
	newBinaryPath := filepath.Join(t.TempDir(), "new-grlx")

	if err := os.WriteFile(currentExe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinaryPath, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}

	stagedPath, err := stageBinary(currentExe, newBinaryPath)
	if err != nil {
		t.Fatalf("stageBinary returned error: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(stagedPath)
	})

	if filepath.Dir(stagedPath) != filepath.Dir(currentExe) {
		t.Fatalf("staged binary directory = %q, want %q", filepath.Dir(stagedPath), filepath.Dir(currentExe))
	}

	contents, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("staged binary contents = %q, want %q", contents, "new")
	}

	info, err := os.Stat(stagedPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("staged binary mode = %v, want %v", got, os.FileMode(0o700))
	}
}

func TestCommitBinaryUpdateReplacesCurrentBinary(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	currentExe := filepath.Join(tempDir, "grlx")
	stagedPath := filepath.Join(tempDir, ".grlx-update-test")

	if err := os.WriteFile(currentExe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := commitBinaryUpdate(currentExe, stagedPath); err != nil {
		t.Fatalf("commitBinaryUpdate returned error: %v", err)
	}

	contents, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("current binary contents = %q, want %q", contents, "new")
	}

	if _, err := os.Stat(currentExe + ".backup"); !os.IsNotExist(err) {
		t.Fatalf("backup stat error = %v, want not exist", err)
	}
}

func TestCommitBinaryUpdateRestoresBackupOnInstallFailure(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	currentExe := filepath.Join(tempDir, "grlx")
	missingStagedPath := filepath.Join(tempDir, ".missing-update")

	if err := os.WriteFile(currentExe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := commitBinaryUpdate(currentExe, missingStagedPath)
	if err == nil {
		t.Fatal("commitBinaryUpdate returned nil error for missing staged binary")
	}
	if !strings.Contains(err.Error(), "failed to install update") {
		t.Fatalf("commitBinaryUpdate error = %q, want install failure", err)
	}

	contents, readErr := os.ReadFile(currentExe)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "old" {
		t.Fatalf("current binary contents = %q, want %q", contents, "old")
	}

	if _, statErr := os.Stat(currentExe + ".backup"); !os.IsNotExist(statErr) {
		t.Fatalf("backup stat error = %v, want not exist", statErr)
	}
}
