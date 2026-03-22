package http

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownload(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("testData"))
	})
	go http.Serve(listener, mux)
	port := listener.Addr().(*net.TCPAddr).Port

	td := t.TempDir()
	type tCase struct {
		name     string
		src      string
		dst      string
		hash     string
		hashType string
		err      error
		ctx      context.Context
	}
	cases := []tCase{{
		name:     "test",
		src:      fmt.Sprintf("http://localhost:%d/test", port),
		dst:      filepath.Join(td, "dst"),
		hash:     "3a760fae784d30a1b50e304e97a17355",
		err:      nil,
		ctx:      context.Background(),
		hashType: "md5",
	}}
	for _, tc := range cases {
		func(tc tCase) {
			t.Run(tc.name, func(t *testing.T) {
				props := make(map[string]interface{})
				props["hashType"] = tc.hashType
				hf, err := (HTTPFile{}).Parse(tc.name, tc.src, tc.dst, tc.hash, props)
				if !errors.Is(err, tc.err) {
					t.Errorf("want error %v, got %v", tc.err, err)
				}
				err = hf.Download(tc.ctx)
				if !errors.Is(err, tc.err) {
					t.Errorf("want error %v, got %v", tc.err, err)
				}
			})
		}(tc)
	}
}

func TestDownloadCustomMethod(t *testing.T) {
	var receivedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Write([]byte("custom-method-data"))
	}))
	defer ts.Close()

	td := t.TempDir()
	props := map[string]interface{}{
		"method": "POST",
	}
	hf := HTTPFile{
		ID:          "custom-method",
		Source:      ts.URL + "/test",
		Destination: filepath.Join(td, "out"),
		Props:       props,
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != "POST" {
		t.Errorf("expected method POST, got %s", receivedMethod)
	}
	data, err := os.ReadFile(filepath.Join(td, "out"))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(data) != "custom-method-data" {
		t.Errorf("unexpected file content: %s", data)
	}
}

func TestDownloadInvalidMethodProp(t *testing.T) {
	// Non-string method prop should fall back to GET
	var receivedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	td := t.TempDir()
	props := map[string]interface{}{
		"method": 12345, // not a string
	}
	hf := HTTPFile{
		ID:          "bad-method",
		Source:      ts.URL + "/test",
		Destination: filepath.Join(td, "out"),
		Props:       props,
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != "GET" {
		t.Errorf("expected fallback to GET, got %s", receivedMethod)
	}
}

func TestDownloadUnexpectedStatusCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "not-found",
		Source:      ts.URL + "/missing",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	err := hf.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 status")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to contain 404, got: %v", err)
	}
	// File should not exist after failure
	if _, statErr := os.Stat(filepath.Join(td, "out")); !os.IsNotExist(statErr) {
		t.Error("destination file should not exist after failed download")
	}
}

func TestDownloadCustomExpectedCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("accepted"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "accepted",
		Source:      ts.URL + "/accept",
		Destination: filepath.Join(td, "out"),
		Props: map[string]interface{}{
			"expectedCode": 202,
		},
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(td, "out"))
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(data) != "accepted" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestDownloadInvalidExpectedCodeProp(t *testing.T) {
	// Non-int expectedCode should fall back to 200
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "bad-expected",
		Source:      ts.URL + "/test",
		Destination: filepath.Join(td, "out"),
		Props: map[string]interface{}{
			"expectedCode": "not-an-int",
		},
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadExpectedCodeMismatch(t *testing.T) {
	// Server returns 200 but we expect 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "code-mismatch",
		Source:      ts.URL + "/test",
		Destination: filepath.Join(td, "out"),
		Props: map[string]interface{}{
			"expectedCode": 201,
		},
	}
	err := hf.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for status code mismatch")
	}
	if !strings.Contains(err.Error(), "200") {
		t.Errorf("expected error to mention actual status code, got: %v", err)
	}
}

func TestDownloadCanceledContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("slow"))
	}))
	defer ts.Close()

	td := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	hf := HTTPFile{
		ID:          "canceled",
		Source:      ts.URL + "/slow",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	err := hf.Download(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestDownloadInvalidURL(t *testing.T) {
	td := t.TempDir()
	hf := HTTPFile{
		ID:          "invalid-url",
		Source:      "http://localhost:1/nonexistent",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	err := hf.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestDownloadBadRequestURL(t *testing.T) {
	td := t.TempDir()
	hf := HTTPFile{
		ID:          "bad-url",
		Source:      "://not-a-url",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	err := hf.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}
}

func TestDownloadDestinationDirNotExist(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer ts.Close()

	hf := HTTPFile{
		ID:          "no-dir",
		Source:      ts.URL + "/test",
		Destination: "/nonexistent/path/file.txt",
		Props:       map[string]interface{}{},
	}
	err := hf.Download(context.Background())
	if err == nil {
		t.Fatal("expected error when destination directory doesn't exist")
	}
}

func TestDownloadServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "server-error",
		Source:      ts.URL + "/error",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	err := hf.Download(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got: %v", err)
	}
}

func TestDownloadNilProps(t *testing.T) {
	// Ensure Download works when Props is nil (no method/expectedCode lookups panic)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "nil-props",
		Source:      ts.URL + "/test",
		Destination: filepath.Join(td, "out"),
		Props:       nil,
	}
	// This will panic if Props is nil — that's a real bug to surface
	// Since the code does hf.Props["method"], nil map read is fine in Go (returns zero value)
	err := hf.Download(context.Background())
	if err != nil {
		t.Fatalf("unexpected error with nil props: %v", err)
	}
}

func TestDownloadLargeBody(t *testing.T) {
	// Test downloading a larger payload to ensure io.Copy works correctly
	largeData := strings.Repeat("x", 1024*1024) // 1MB
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(largeData))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "large",
		Source:      ts.URL + "/large",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(td, "out"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if len(data) != len(largeData) {
		t.Errorf("expected %d bytes, got %d", len(largeData), len(data))
	}
}

func TestProperties(t *testing.T) {
	props := map[string]interface{}{
		"hashType": "sha256",
		"method":   "PUT",
		"custom":   "value",
	}
	hf := HTTPFile{
		ID:    "props-test",
		Props: props,
	}
	got, err := hf.Properties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["hashType"] != "sha256" {
		t.Errorf("expected hashType sha256, got %v", got["hashType"])
	}
	if got["method"] != "PUT" {
		t.Errorf("expected method PUT, got %v", got["method"])
	}
	if got["custom"] != "value" {
		t.Errorf("expected custom value, got %v", got["custom"])
	}
}

func TestPropertiesNil(t *testing.T) {
	hf := HTTPFile{}
	got, err := hf.Properties()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil props, got %v", got)
	}
}

func TestParse(t *testing.T) {
	t.Run("with properties", func(t *testing.T) {
		props := map[string]interface{}{"key": "val"}
		provider, err := (HTTPFile{}).Parse("myid", "http://example.com/f", "/tmp/dest", "abc123", props)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		hf, ok := provider.(HTTPFile)
		if !ok {
			t.Fatal("expected HTTPFile type")
		}
		if hf.ID != "myid" {
			t.Errorf("expected ID myid, got %s", hf.ID)
		}
		if hf.Source != "http://example.com/f" {
			t.Errorf("unexpected Source: %s", hf.Source)
		}
		if hf.Destination != "/tmp/dest" {
			t.Errorf("unexpected Destination: %s", hf.Destination)
		}
		if hf.Hash != "abc123" {
			t.Errorf("unexpected Hash: %s", hf.Hash)
		}
		if hf.Props["key"] != "val" {
			t.Errorf("unexpected Props: %v", hf.Props)
		}
	})

	t.Run("nil properties", func(t *testing.T) {
		provider, err := (HTTPFile{}).Parse("id2", "http://example.com", "/tmp/out", "hash", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		hf, ok := provider.(HTTPFile)
		if !ok {
			t.Fatal("expected HTTPFile type")
		}
		if hf.Props == nil {
			t.Error("expected non-nil Props after parsing with nil properties")
		}
	})
}

func TestProtocols(t *testing.T) {
	hf := HTTPFile{}
	protocols := hf.Protocols()
	if len(protocols) != 2 {
		t.Fatalf("expected 2 protocols, got %d", len(protocols))
	}
	found := make(map[string]bool)
	for _, p := range protocols {
		found[p] = true
	}
	if !found["http"] {
		t.Error("expected http protocol")
	}
	if !found["https"] {
		t.Error("expected https protocol")
	}
}

func TestVerify(t *testing.T) {
	td := t.TempDir()
	content := []byte("verify this content")

	// Compute hashes
	md5Sum := md5.Sum(content)
	md5Hex := hex.EncodeToString(md5Sum[:])
	sha256Sum := sha256.Sum256(content)
	sha256Hex := hex.EncodeToString(sha256Sum[:])
	sha512Sum := sha512.Sum512(content)
	sha512Hex := hex.EncodeToString(sha512Sum[:])

	tests := []struct {
		name     string
		hash     string
		hashType interface{} // can be string or nil or non-string
		wantOK   bool
	}{
		{
			name:     "md5 explicit",
			hash:     md5Hex,
			hashType: "md5",
			wantOK:   true,
		},
		{
			name:     "sha256 explicit",
			hash:     sha256Hex,
			hashType: "sha256",
			wantOK:   true,
		},
		{
			name:     "sha512 explicit",
			hash:     sha512Hex,
			hashType: "sha512",
			wantOK:   true,
		},
		{
			name:     "hash mismatch",
			hash:     "0000000000000000000000000000000000000000000000000000000000000000",
			hashType: "sha256",
			wantOK:   false,
		},
		{
			name:     "nil hashType guesses from length",
			hash:     sha256Hex,
			hashType: nil,
			wantOK:   true,
		},
		{
			name:     "non-string hashType guesses from length",
			hash:     md5Hex,
			hashType: 42,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := filepath.Join(td, tt.name)
			if err := os.WriteFile(dest, content, 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			props := map[string]interface{}{}
			if tt.hashType != nil {
				props["hashType"] = tt.hashType
			}

			hf := HTTPFile{
				ID:          tt.name,
				Destination: dest,
				Hash:        tt.hash,
				Props:       props,
			}

			ok, err := hf.Verify(context.Background())
			if tt.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !ok {
					t.Error("expected Verify to return true")
				}
			} else {
				if ok {
					t.Error("expected Verify to return false")
				}
			}
		})
	}
}

func TestVerifyFileNotExist(t *testing.T) {
	hf := HTTPFile{
		ID:          "missing",
		Destination: "/nonexistent/path/file.txt",
		Hash:        "abc123",
		Props:       map[string]interface{}{},
	}
	ok, err := hf.Verify(context.Background())
	if ok {
		t.Error("expected Verify to return false for missing file")
	}
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestVerifyNilHashType(t *testing.T) {
	td := t.TempDir()
	content := []byte("test content for nil hashType")
	dest := filepath.Join(td, "testfile")
	if err := os.WriteFile(dest, content, 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	sha256Sum := sha256.Sum256(content)
	sha256Hex := hex.EncodeToString(sha256Sum[:])

	// Props with no hashType key at all
	hf := HTTPFile{
		ID:          "nil-ht",
		Destination: dest,
		Hash:        sha256Hex,
		Props:       map[string]interface{}{},
	}
	ok, err := hf.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected Verify to return true")
	}
}

func TestDownloadEmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "empty-body",
		Source:      ts.URL + "/empty",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(td, "out"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestDownloadRedirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.Write([]byte("redirected"))
	}))
	defer ts.Close()

	td := t.TempDir()
	hf := HTTPFile{
		ID:          "redirect",
		Source:      ts.URL + "/redirect",
		Destination: filepath.Join(td, "out"),
		Props:       map[string]interface{}{},
	}
	if err := hf.Download(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(td, "out"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(data) != "redirected" {
		t.Errorf("expected 'redirected', got %q", data)
	}
}
