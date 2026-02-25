package hashers

import (
	"io"
	"strings"
	"testing"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

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
}

func TestRegisterDuplicate(t *testing.T) {
	err := Register("md5", MD5)
	if err == nil {
		t.Error("expected error when registering duplicate hash func")
	}
}
