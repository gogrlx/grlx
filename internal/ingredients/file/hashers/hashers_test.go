package hashers

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

// errReader always returns an error on Read.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, errors.New("read error") }
func (errReader) Close() error               { return nil }

// --- Hash function tests ---

func TestMD5(t *testing.T) {
	// md5 of "hello\n" = b1946ac92492d2347c6235b4d2611184
	r := nopCloser{strings.NewReader("hello\n")}
	hash, match, err := MD5(r, "b1946ac92492d2347c6235b4d2611184")
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match, got hash %s", hash)
	}
}

func TestMD5Mismatch(t *testing.T) {
	r := nopCloser{strings.NewReader("hello\n")}
	hash, match, err := MD5(r, "0000000000000000000000000000000a")
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Errorf("expected mismatch, got hash %s", hash)
	}
	if hash != "b1946ac92492d2347c6235b4d2611184" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestMD5ReadError(t *testing.T) {
	r := errReader{}
	_, _, err := MD5(r, "anything")
	if err == nil {
		t.Error("expected error on read failure")
	}
}

func TestMD5Empty(t *testing.T) {
	// md5 of empty string = d41d8cd98f00b204e9800998ecf8427e
	r := nopCloser{strings.NewReader("")}
	hash, match, err := MD5(r, "d41d8cd98f00b204e9800998ecf8427e")
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match for empty input, got hash %s", hash)
	}
}

func TestSHA1(t *testing.T) {
	// sha1 of "hello\n"
	r := nopCloser{strings.NewReader("hello\n")}
	hash, match, err := SHA1(r, "f572d396fae9206628714fb2ce00f72e94f2258f")
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match, got hash %s", hash)
	}
}

func TestSHA1Mismatch(t *testing.T) {
	r := nopCloser{strings.NewReader("hello\n")}
	_, match, err := SHA1(r, "0000000000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected mismatch")
	}
}

func TestSHA1ReadError(t *testing.T) {
	r := errReader{}
	_, _, err := SHA1(r, "anything")
	if err == nil {
		t.Error("expected error on read failure")
	}
}

func TestSHA256(t *testing.T) {
	// sha256 of "hello\n" = 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	r := nopCloser{strings.NewReader("hello\n")}
	hash, match, err := SHA256(r, "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match, got hash %s", hash)
	}
}

func TestSHA256Mismatch(t *testing.T) {
	r := nopCloser{strings.NewReader("hello\n")}
	_, match, err := SHA256(r, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected mismatch")
	}
}

func TestSHA256ReadError(t *testing.T) {
	r := errReader{}
	_, _, err := SHA256(r, "anything")
	if err == nil {
		t.Error("expected error on read failure")
	}
}

func TestSHA512(t *testing.T) {
	// sha512 of "hello\n"
	r := nopCloser{strings.NewReader("hello\n")}
	hash, match, err := SHA512(r, "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629")
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match, got hash %s", hash)
	}
}

func TestSHA512Mismatch(t *testing.T) {
	r := nopCloser{strings.NewReader("hello\n")}
	_, match, err := SHA512(r, strings.Repeat("0", 128))
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected mismatch")
	}
}

func TestSHA512ReadError(t *testing.T) {
	r := errReader{}
	_, _, err := SHA512(r, "anything")
	if err == nil {
		t.Error("expected error on read failure")
	}
}

func TestCRC32(t *testing.T) {
	// crc32 of "hello\n" using IEEE table
	r1 := nopCloser{strings.NewReader("hello\n")}
	hash, _, err := CRC32(r1, "")
	if err != nil {
		t.Fatal(err)
	}
	// Now verify match
	r2 := nopCloser{strings.NewReader("hello\n")}
	_, match, err := CRC32(r2, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match for CRC32")
	}
}

func TestCRC32Mismatch(t *testing.T) {
	r := nopCloser{strings.NewReader("hello\n")}
	_, match, err := CRC32(r, "00000000")
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Error("expected mismatch")
	}
}

func TestCRC32ReadError(t *testing.T) {
	r := errReader{}
	_, _, err := CRC32(r, "anything")
	if err == nil {
		t.Error("expected error on read failure")
	}
}

// --- GetHashFunc tests ---

func TestGetHashFunc(t *testing.T) {
	for _, name := range []string{"md5", "sha1", "sha256", "sha512", "crc"} {
		_, err := GetHashFunc(name)
		if err != nil {
			t.Errorf("expected hash func %s to be registered, got error: %v", name, err)
		}
	}
	_, err := GetHashFunc("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent hash func")
	}
	if !errors.Is(err, ErrorHashFuncNotFound) {
		t.Errorf("expected ErrorHashFuncNotFound, got: %v", err)
	}
}

// --- Register tests ---

func TestRegisterDuplicate(t *testing.T) {
	err := Register("md5", MD5)
	if err == nil {
		t.Error("expected error when registering duplicate hash func")
	}
	if !errors.Is(err, ErrHashFuncExists) {
		t.Errorf("expected ErrHashFuncExists, got: %v", err)
	}
}

func TestRegisterNew(t *testing.T) {
	customFunc := func(r io.ReadCloser, expected string) (string, bool, error) {
		defer r.Close()
		return "custom", expected == "custom", nil
	}
	err := Register("custom-test-hash", customFunc)
	if err != nil {
		t.Fatalf("unexpected error registering new hash func: %v", err)
	}
	hf, err := GetHashFunc("custom-test-hash")
	if err != nil {
		t.Fatalf("expected to find registered hash func: %v", err)
	}
	r := nopCloser{strings.NewReader("test")}
	hash, match, err := hf(r, "custom")
	if err != nil {
		t.Fatal(err)
	}
	if !match || hash != "custom" {
		t.Errorf("custom hash func returned unexpected values: hash=%s match=%v", hash, match)
	}
}

// --- GuessHashType tests ---

func TestGuessHashType(t *testing.T) {
	tests := []struct {
		hash     string
		expected string
	}{
		{strings.Repeat("a", 32), "md5"},
		{strings.Repeat("a", 40), "sha1"},
		{strings.Repeat("a", 64), "sha256"},
		{strings.Repeat("a", 128), "sha512"},
		{strings.Repeat("a", 8), "crc"},
		{strings.Repeat("a", 16), "unknown"},
		{strings.Repeat("a", 1), "unknown"},
		// Prefixed hash types
		{"md5:b1946ac92492d2347c6235b4d2611184", "md5"},
		{"sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "sha256"},
		{"sha512:" + strings.Repeat("a", 128), "sha512"},
		{"custom:whatever", "custom"},
	}
	for _, tc := range tests {
		got := GuessHashType(tc.hash)
		if got != tc.expected {
			t.Errorf("GuessHashType(%q): expected %q, got %q", tc.hash, tc.expected, got)
		}
	}
}

// --- FileToReader tests ---

func TestFileToReader(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "testfile.txt")
	content := "test content for hashing\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, err := FileToReader(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestFileToReaderMissing(t *testing.T) {
	_, err := FileToReader("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- CacheFile.Verify tests ---

func TestCacheFileVerifySuccess(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.txt")
	content := "hello\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cf := CacheFile{
		ID:          "step-1",
		Destination: filePath,
		Hash:        "b1946ac92492d2347c6235b4d2611184",
		HashType:    "md5",
	}
	match, err := cf.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !match {
		t.Error("expected hash to match")
	}
}

func TestCacheFileVerifyMismatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cf := CacheFile{
		ID:          "step-1",
		Destination: filePath,
		Hash:        "0000000000000000000000000000000a",
		HashType:    "md5",
	}
	match, err := cf.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match {
		t.Error("expected hash mismatch")
	}
}

func TestCacheFileVerifyFileNotFound(t *testing.T) {
	cf := CacheFile{
		ID:          "step-1",
		Destination: "/nonexistent/path/file.txt",
		Hash:        "anything",
		HashType:    "md5",
	}
	match, err := cf.Verify(context.Background())
	if err == nil {
		t.Error("expected error for missing file")
	}
	if match {
		t.Error("expected no match for missing file")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got: %v", err)
	}
}

func TestCacheFileVerifyGuessHashType(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Empty HashType should trigger GuessHashType based on hash length (32 = md5)
	cf := CacheFile{
		ID:          "step-1",
		Destination: filePath,
		Hash:        "b1946ac92492d2347c6235b4d2611184",
		HashType:    "",
	}
	match, err := cf.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !match {
		t.Error("expected match with guessed hash type")
	}
}

func TestCacheFileVerifyPrefixedHash(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Hash with "md5:" prefix — Verify should strip the prefix
	cf := CacheFile{
		ID:          "step-1",
		Destination: filePath,
		Hash:        "md5:b1946ac92492d2347c6235b4d2611184",
		HashType:    "",
	}
	match, err := cf.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !match {
		t.Error("expected match with prefixed hash")
	}
}

func TestCacheFileVerifyUnknownHashType(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cf := CacheFile{
		ID:          "step-1",
		Destination: filePath,
		Hash:        "abc",
		HashType:    "nonexistent-algo",
	}
	_, err := cf.Verify(context.Background())
	if err == nil {
		t.Error("expected error for unknown hash type")
	}
	if !errors.Is(err, ErrorHashFuncNotFound) {
		t.Errorf("expected ErrorHashFuncNotFound, got: %v", err)
	}
}

func TestCacheFileVerifyFileOpenError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Remove read permission
	if err := os.Chmod(filePath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filePath, 0o644) })

	cf := CacheFile{
		ID:          "step-1",
		Destination: filePath,
		Hash:        "b1946ac92492d2347c6235b4d2611184",
		HashType:    "md5",
	}
	match, err := cf.Verify(context.Background())
	if err == nil {
		t.Error("expected error for unreadable file")
	}
	if match {
		t.Error("expected no match")
	}
	// Should not be ErrFileNotFound — file exists but can't be opened
	if errors.Is(err, ErrFileNotFound) {
		t.Error("should not be ErrFileNotFound for permission error")
	}
}

func TestCacheFileVerifyWithSHA256(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cf := CacheFile{
		ID:          "step-2",
		Destination: filePath,
		Hash:        "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		HashType:    "sha256",
	}
	match, err := cf.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !match {
		t.Error("expected SHA256 hash to match")
	}
}

// --- Integration: hash real files ---

func TestHashRealFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "realfile.txt")
	content := "the quick brown fox jumps over the lazy dog\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, err := FileToReader(filePath)
	if err != nil {
		t.Fatal(err)
	}
	hash, _, err := SHA256(rc, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64 char SHA256 hash, got %d chars: %s", len(hash), hash)
	}

	// Verify the hash matches on re-read
	rc2, err := FileToReader(filePath)
	if err != nil {
		t.Fatal(err)
	}
	_, match, err := SHA256(rc2, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("expected re-hashed file to match")
	}
}

func TestHashEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(filePath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	rc, err := FileToReader(filePath)
	if err != nil {
		t.Fatal(err)
	}
	// SHA256 of empty content = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	hash, match, err := SHA256(rc, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Errorf("expected match for empty file SHA256, got %s", hash)
	}
}
