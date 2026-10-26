package hashers

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheFileVerifyCanceledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cached")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cached := CacheFile{Destination: path, Hash: "md5:b1946ac92492d2347c6235b4d2611184"}
	matched, err := cached.Verify(ctx)
	if matched || !errors.Is(err, context.Canceled) {
		t.Fatalf("Verify() = %v, %v; want false, context.Canceled", matched, err)
	}
}

func TestCacheFileVerifyCanceledDuringRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cached")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hashType := t.Name()
	err := Register(hashType, func(reader io.ReadCloser, expected string) (string, bool, error) {
		defer reader.Close()
		first := make([]byte, 1)
		if _, err := io.ReadFull(reader, first); err != nil {
			return "", false, err
		}
		cancel()
		_, err := io.Copy(io.Discard, reader)
		return "", err == nil, err
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hfTex.Lock(); delete(hashFuncs, hashType); hfTex.Unlock() })
	cached := CacheFile{Destination: path, HashType: hashType}
	matched, err := cached.Verify(ctx)
	if matched || !errors.Is(err, context.Canceled) {
		t.Fatalf("Verify() = %v, %v; want false, context.Canceled", matched, err)
	}
}
