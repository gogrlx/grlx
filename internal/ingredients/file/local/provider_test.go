package local

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/ingredients/file"
	"github.com/gogrlx/grlx/v2/internal/ingredients/file/hashers"
)

func TestProtocols(t *testing.T) {
	lf := LocalFile{}
	protos := lf.Protocols()
	if len(protos) != 1 || protos[0] != "file" {
		t.Errorf("expected [\"file\"], got %v", protos)
	}
}

func TestParse(t *testing.T) {
	lf := LocalFile{}
	cases := []struct {
		name  string
		id    string
		src   string
		dst   string
		hash  string
		props map[string]interface{}
	}{
		{
			name:  "basic parse",
			id:    "step1",
			src:   "/tmp/src",
			dst:   "/tmp/dst",
			hash:  "abc123",
			props: map[string]interface{}{"hashType": "md5"},
		},
		{
			name:  "nil props",
			id:    "step2",
			src:   "/tmp/src",
			dst:   "/tmp/dst",
			hash:  "def456",
			props: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fp, err := lf.Parse(tc.id, tc.src, tc.dst, tc.hash, tc.props)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			parsed, ok := fp.(LocalFile)
			if !ok {
				t.Fatal("expected LocalFile type")
			}
			if parsed.ID != tc.id {
				t.Errorf("ID: want %q, got %q", tc.id, parsed.ID)
			}
			if parsed.Source != tc.src {
				t.Errorf("Source: want %q, got %q", tc.src, parsed.Source)
			}
			if parsed.Destination != tc.dst {
				t.Errorf("Destination: want %q, got %q", tc.dst, parsed.Destination)
			}
			if parsed.Hash != tc.hash {
				t.Errorf("Hash: want %q, got %q", tc.hash, parsed.Hash)
			}
			if parsed.Props == nil {
				t.Error("Props should not be nil (nil input should be initialized)")
			}
		})
	}
}

func TestProperties(t *testing.T) {
	props := map[string]interface{}{"hashType": "sha256", "mode": "0644"}
	lf := LocalFile{Props: props}
	got, err := lf.Properties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["hashType"] != "sha256" {
		t.Errorf("hashType: want sha256, got %v", got["hashType"])
	}
	if got["mode"] != "0644" {
		t.Errorf("mode: want 0644, got %v", got["mode"])
	}
}

func TestDownloadCopiesFile(t *testing.T) {
	td := t.TempDir()
	srcPath := filepath.Join(td, "source.txt")
	dstPath := filepath.Join(td, "destination.txt")
	content := []byte("hello grlx local provider")

	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute MD5 hash of destination after copy
	h := md5.Sum(content)
	hash := fmt.Sprintf("%x", h)

	lf := LocalFile{
		ID:          "test-download",
		Source:      srcPath,
		Destination: dstPath,
		Hash:        hash,
		Props:       map[string]interface{}{"hashType": "md5"},
	}

	ctx := context.Background()
	if err := lf.Download(ctx); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: want %q, got %q", content, got)
	}
}

func TestDownloadSkipsWhenHashMatches(t *testing.T) {
	td := t.TempDir()
	srcPath := filepath.Join(td, "source.txt")
	dstPath := filepath.Join(td, "destination.txt")
	content := []byte("already here")

	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-populate destination with same content
	if err := os.WriteFile(dstPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := md5.Sum(content)
	hash := fmt.Sprintf("%x", h)

	lf := LocalFile{
		ID:          "test-skip",
		Source:      srcPath,
		Destination: dstPath,
		Hash:        hash,
		Props:       map[string]interface{}{"hashType": "md5"},
	}

	ctx := context.Background()
	if err := lf.Download(ctx); err != nil {
		t.Fatalf("Download failed when dest already matches: %v", err)
	}
}

func TestDownloadSourceNotFound(t *testing.T) {
	td := t.TempDir()
	dstPath := filepath.Join(td, "destination.txt")

	lf := LocalFile{
		ID:          "test-missing-src",
		Source:      filepath.Join(td, "nonexistent.txt"),
		Destination: dstPath,
		Hash:        "abc123",
		Props:       map[string]interface{}{},
	}

	ctx := context.Background()
	err := lf.Download(ctx)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
	if !os.IsNotExist(err) {
		// The error wraps os.Open's not-exist error
		if !errors.Is(err, os.ErrNotExist) {
			t.Logf("got error (acceptable): %v", err)
		}
	}
}

func TestVerifyMatchingHash(t *testing.T) {
	td := t.TempDir()
	filePath := filepath.Join(td, "test.txt")
	content := []byte("verify me")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	hash := fmt.Sprintf("%x", h)

	lf := LocalFile{
		ID:          "test-verify",
		Destination: filePath,
		Hash:        hash,
		Props:       map[string]interface{}{"hashType": "sha256"},
	}

	ctx := context.Background()
	ok, err := lf.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("expected Verify to return true for matching hash")
	}
}

func TestVerifyMismatchedHash(t *testing.T) {
	td := t.TempDir()
	filePath := filepath.Join(td, "test.txt")
	if err := os.WriteFile(filePath, []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}

	lf := LocalFile{
		ID:          "test-verify-mismatch",
		Destination: filePath,
		Hash:        "0000000000000000000000000000000000000000000000000000000000000000",
		Props:       map[string]interface{}{"hashType": "sha256"},
	}

	ctx := context.Background()
	ok, err := lf.Verify(ctx)
	if ok {
		t.Error("expected Verify to return false for mismatched hash")
	}
	// err may or may not be nil depending on implementation — hash mismatch
	// can be signaled by ok=false alone or with an error
	_ = err
}

func TestVerifyFileNotFound(t *testing.T) {
	td := t.TempDir()
	lf := LocalFile{
		ID:          "test-verify-missing",
		Destination: filepath.Join(td, "nonexistent.txt"),
		Hash:        "abc123",
		Props:       map[string]interface{}{"hashType": "md5"},
	}

	ctx := context.Background()
	ok, err := lf.Verify(ctx)
	if ok {
		t.Error("expected Verify to return false for missing file")
	}
	if !errors.Is(err, hashers.ErrFileNotFound) && !errors.Is(err, file.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got: %v", err)
	}
}

func TestVerifyGuessesHashType(t *testing.T) {
	td := t.TempDir()
	filePath := filepath.Join(td, "test.txt")
	content := []byte("guess my hash")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// MD5 hash (32 chars) — should be auto-detected
	h := md5.Sum(content)
	hash := fmt.Sprintf("%x", h)

	lf := LocalFile{
		ID:          "test-guess-hash",
		Destination: filePath,
		Hash:        hash,
		Props:       map[string]interface{}{}, // no hashType set
	}

	ctx := context.Background()
	ok, err := lf.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("expected Verify to succeed with auto-detected MD5 hash type")
	}
}

func TestDownloadWithSHA256(t *testing.T) {
	td := t.TempDir()
	srcPath := filepath.Join(td, "source.txt")
	dstPath := filepath.Join(td, "destination.txt")
	content := []byte("sha256 download test")

	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	hash := fmt.Sprintf("%x", h)

	lf := LocalFile{
		ID:          "test-sha256",
		Source:      srcPath,
		Destination: dstPath,
		Hash:        hash,
		Props:       map[string]interface{}{"hashType": "sha256"},
	}

	ctx := context.Background()
	if err := lf.Download(ctx); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	ok, err := lf.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("expected Verify to pass after Download")
	}
}
