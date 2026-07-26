package serve

import (
	"io/fs"
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
	const fallbackPath = "/sprouts/abc123"
	req := httptest.NewRequest(http.MethodGet, fallbackPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA fallback, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "grlx") {
		t.Fatal("expected SPA fallback to serve index.html content")
	}
	if req.URL.Path != fallbackPath {
		t.Fatalf("expected fallback to preserve request path %q, got %q", fallbackPath, req.URL.Path)
	}
}

func TestUIHandler_AssetsCacheHeaders(t *testing.T) {
	handler := UIHandler()

	assetPath := findEmbeddedAsset(t)
	req := httptest.NewRequest(http.MethodGet, "/"+assetPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected asset status 200, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache header, got %q", cc)
	}
}

func TestUIHandler_IndexDoesNotUseImmutableCache(t *testing.T) {
	handler := UIHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if cc := rec.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Fatal("root path should not have immutable cache header")
	}
}

func findEmbeddedAsset(t *testing.T) string {
	t.Helper()

	distFS, err := fs.Sub(uiAssets, "dist")
	if err != nil {
		t.Fatalf("open embedded dist filesystem: %v", err)
	}

	var assetPath string
	err = fs.WalkDir(distFS, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && assetPath == "" {
			assetPath = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded assets: %v", err)
	}
	if assetPath == "" {
		t.Fatal("expected at least one embedded asset")
	}
	return assetPath
}
