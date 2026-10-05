//go:build self_update
// +build self_update

package selfupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
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

func TestParseLatestVersionRejectsUnstableReleases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "draft",
			body: `{"tag_name":"v1.2.3","draft":true}`,
			want: "draft",
		},
		{
			name: "prerelease",
			body: `{"tag_name":"v1.2.3","prerelease":true}`,
			want: "prerelease",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseLatestVersion(strings.NewReader(test.body))
			if err == nil {
				t.Fatal("parseLatestVersion returned nil error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseLatestVersion error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestParseLatestVersionRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	body := strings.NewReader(strings.Repeat(" ", maxLatestReleaseResponseBytes+1))
	_, err := parseLatestVersion(body)
	if err == nil {
		t.Fatal("parseLatestVersion returned nil error for oversized response")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("parseLatestVersion error = %q, want size limit failure", err)
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

func TestVerifyChecksumsSignatureAcceptsTrustedSigner(t *testing.T) {
	t.Parallel()

	publicKey, fingerprint, signature := testSignedChecksums(t, "checksum manifest")

	err := verifyChecksumsSignature(
		strings.NewReader(publicKey),
		strings.NewReader("checksum manifest"),
		bytes.NewReader(signature),
		fingerprint,
	)
	if err != nil {
		t.Fatalf("verifyChecksumsSignature returned error: %v", err)
	}
}

func TestVerifyChecksumsSignatureRejectsTamperedChecksums(t *testing.T) {
	t.Parallel()

	publicKey, fingerprint, signature := testSignedChecksums(t, "checksum manifest")

	err := verifyChecksumsSignature(
		strings.NewReader(publicKey),
		strings.NewReader("tampered manifest"),
		bytes.NewReader(signature),
		fingerprint,
	)
	if err == nil {
		t.Fatal("verifyChecksumsSignature returned nil error for tampered checksums")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Fatalf("verifyChecksumsSignature error = %q, want signature failure", err)
	}
}

func TestVerifyChecksumsSignatureRejectsUnexpectedFingerprint(t *testing.T) {
	t.Parallel()

	publicKey, _, signature := testSignedChecksums(t, "checksum manifest")

	err := verifyChecksumsSignature(
		strings.NewReader(publicKey),
		strings.NewReader("checksum manifest"),
		bytes.NewReader(signature),
		"3F62 7C68 8B72 ACC6 BC4C A9A7 1E0B 7A1D 33DC E4DD",
	)
	if err == nil {
		t.Fatal("verifyChecksumsSignature returned nil error for unexpected signer")
	}
	if !strings.Contains(err.Error(), "unexpected key fingerprint") {
		t.Fatalf("verifyChecksumsSignature error = %q, want unexpected fingerprint failure", err)
	}
}

func TestFingerprintMatchesDocumentedFormat(t *testing.T) {
	t.Parallel()

	fingerprint, err := hex.DecodeString("3f627c688b72acc6bc4ca9a71e0b7a1d33dce4dd")
	if err != nil {
		t.Fatal(err)
	}

	if !fingerprintMatches(fingerprint, "3F62 7C68 8B72 ACC6 BC4C A9A7 1E0B 7A1D 33DC E4DD") {
		t.Fatal("fingerprintMatches rejected documented spaced uppercase fingerprint format")
	}
}

func TestChecksumForArtifactRejectsMissingName(t *testing.T) {
	t.Parallel()

	_, err := checksumForArtifact(strings.NewReader(""), "")
	if err == nil {
		t.Fatal("checksumForArtifact returned nil error for missing artifact name")
	}
}

func testSignedChecksums(t *testing.T, checksums string) (string, string, []byte) {
	t.Helper()

	entity, err := openpgp.NewEntity("grlx signing key", "", "security@grlx.dev", nil)
	if err != nil {
		t.Fatalf("NewEntity returned error: %v", err)
	}

	var publicKey bytes.Buffer
	keyWriter, err := armor.Encode(&publicKey, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatalf("armor.Encode returned error: %v", err)
	}
	if err := entity.Serialize(keyWriter); err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	if err := keyWriter.Close(); err != nil {
		t.Fatalf("closing armored public key returned error: %v", err)
	}

	var signature bytes.Buffer
	if err := openpgp.DetachSign(&signature, entity, strings.NewReader(checksums), nil); err != nil {
		t.Fatalf("DetachSign returned error: %v", err)
	}

	return publicKey.String(), fingerprintString(entity.PrimaryKey.Fingerprint[:]), signature.Bytes()
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

	matches, err := filepath.Glob(currentExe + ".backup-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("backup files = %v, want none", matches)
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

	matches, err := filepath.Glob(currentExe + ".backup-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("backup files = %v, want none", matches)
	}
}

func TestCommitBinaryUpdateDoesNotClobberExistingBackup(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	currentExe := filepath.Join(tempDir, "grlx")
	existingBackup := currentExe + ".backup"
	missingStagedPath := filepath.Join(tempDir, ".missing-update")

	if err := os.WriteFile(currentExe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingBackup, []byte("previous backup"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := commitBinaryUpdate(currentExe, missingStagedPath)
	if err == nil {
		t.Fatal("commitBinaryUpdate returned nil error for missing staged binary")
	}

	contents, err := os.ReadFile(existingBackup)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "previous backup" {
		t.Fatalf("existing backup contents = %q, want %q", contents, "previous backup")
	}
}
