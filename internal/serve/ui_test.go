package serve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUIHandler_ServesIndexHTML(t *testing.T) {
	handler := UIHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "grlx") {
		t.Fatal("expected index.html to contain 'grlx'")
	}
}

func TestUIHandler_SPAFallback(t *testing.T) {
	handler := UIHandler()

	// Request a path that doesn't exist as a file — should get index.html
	req := httptest.NewRequest(http.MethodGet, "/sprouts/abc123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA fallback, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "grlx") {
		t.Fatal("expected SPA fallback to serve index.html content")
	}
}

func TestUIHandler_AssetsCacheHeaders(t *testing.T) {
	handler := UIHandler()

	// The placeholder dist has an assets/ directory but no files in it.
	// Just verify a non-asset path doesn't get the immutable cache header.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Fatal("root path should not have immutable cache header")
	}
}
