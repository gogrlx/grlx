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

func TestChecksumNameUsesExplicitConfig(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{
		BinaryName:   "/usr/local/bin/grlx",
		ChecksumName: "grlx-v1.2.3-linux-amd64",
	})

	if got := updater.checksumName(); got != "grlx-v1.2.3-linux-amd64" {
		t.Fatalf("checksumName() = %q, want grlx-v1.2.3-linux-amd64", got)
	}
}

func TestChecksumNameDefaultsToBinaryBase(t *testing.T) {
	t.Parallel()

	updater := NewUpdater(UpdateConfig{BinaryName: "/usr/local/bin/grlx"})

	if got := updater.checksumName(); got != "grlx" {
		t.Fatalf("checksumName() = %q, want grlx", got)
	}
}

func TestVerifyArtifactChecksumAcceptsMatchingChecksum(t *testing.T) {
	t.Parallel()

	artifact := "new binary"
	checksum := sha256.Sum256([]byte(artifact))
	manifest := fmt.Sprintf("%x  dist/grlx-v1.2.3-linux-amd64\n", checksum)

	err := verifyArtifactChecksum(strings.NewReader(manifest), strings.NewReader(artifact), "grlx-v1.2.3-linux-amd64")
	if err != nil {
		t.Fatalf("verifyArtifactChecksum returned error: %v", err)
	}
}

func TestVerifyArtifactChecksumRejectsMismatch(t *testing.T) {
	t.Parallel()

	checksum := sha256.Sum256([]byte("expected binary"))
	manifest := fmt.Sprintf("%x  grlx-v1.2.3-linux-amd64\n", checksum)

	err := verifyArtifactChecksum(strings.NewReader(manifest), strings.NewReader("different binary"), "grlx-v1.2.3-linux-amd64")
	if err == nil {
		t.Fatal("verifyArtifactChecksum returned nil error for mismatched artifact")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("verifyArtifactChecksum error = %q, want checksum mismatch", err)
	}
}

func TestChecksumForArtifactRejectsMissingName(t *testing.T) {
	t.Parallel()

	_, err := checksumForArtifact(strings.NewReader(""), "")
	if err == nil {
		t.Fatal("checksumForArtifact returned nil error for missing artifact name")
	}
}

func TestChecksumForArtifactRejectsMissingChecksum(t *testing.T) {
	t.Parallel()

	checksum := sha256.Sum256([]byte("other binary"))
	manifest := fmt.Sprintf("%x  grlx-v1.2.3-darwin-amd64\n", checksum)

	_, err := checksumForArtifact(strings.NewReader(manifest), "grlx-v1.2.3-linux-amd64")
	if err == nil {
		t.Fatal("checksumForArtifact returned nil error for missing checksum")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("checksumForArtifact error = %q, want not found", err)
	}
}

func TestChecksumForArtifactIgnoresMalformedEntries(t *testing.T) {
	t.Parallel()

	artifact := "new binary"
	checksum := sha256.Sum256([]byte(artifact))
	manifest := fmt.Sprintf("not-a-checksum  grlx-v1.2.3-linux-amd64\n%x  *grlx-v1.2.3-linux-amd64\n", checksum)

	got, err := checksumForArtifact(strings.NewReader(manifest), "grlx-v1.2.3-linux-amd64")
	if err != nil {
		t.Fatalf("checksumForArtifact returned error: %v", err)
	}
	if want := fmt.Sprintf("%x", checksum); got != want {
		t.Fatalf("checksumForArtifact = %q, want %q", got, want)
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
