//go:build self_update
// +build self_update

package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
