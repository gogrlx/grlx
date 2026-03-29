package serve

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithGzip_CompressesHTML(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		// Write enough to trigger sniff buffer flush.
		w.Write([]byte(strings.Repeat("<p>hello world</p>\n", 100)))
	})

	handler := WithGzip(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("expected Content-Encoding: gzip, got %q", ce)
	}

	// Verify we can decompress the body.
	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	body, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if !strings.Contains(string(body), "hello world") {
		t.Fatalf("body missing expected content")
	}
}

func TestWithGzip_SkipsWhenNoAcceptEncoding(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<p>raw</p>"))
	})

	handler := WithGzip(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header.
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if ce := rec.Header().Get("Content-Encoding"); ce != "" {
		t.Fatalf("expected no Content-Encoding, got %q", ce)
	}
	if !strings.Contains(rec.Body.String(), "raw") {
		t.Fatalf("body missing expected content")
	}
}

func TestWithGzip_SkipsBinaryContent(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(append(pngHeader, make([]byte, 600)...))
	})

	handler := WithGzip(inner)
	req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if ce := rec.Header().Get("Content-Encoding"); ce == "gzip" {
		t.Fatalf("should not gzip image/png")
	}
}

func TestWithGzip_CompressesJSON(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(strings.Repeat(`{"key":"value"}`, 100)))
	})

	handler := WithGzip(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if ce := rec.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("expected gzip for JSON, got %q", ce)
	}

	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	body, _ := io.ReadAll(gr)
	if !strings.Contains(string(body), "key") {
		t.Fatalf("body missing expected JSON content")
	}
}

func TestWithGzip_SetsVaryHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Repeat("test content\n", 100)))
	})

	handler := WithGzip(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if v := rec.Header().Get("Vary"); !strings.Contains(v, "Accept-Encoding") {
		t.Fatalf("expected Vary: Accept-Encoding, got %q", v)
	}
}
