package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func TestNewRouterHealthEndpoint(t *testing.T) {
	// Set up a temporary recipe directory so the file server has
	// a valid root (NewRouter reads config.RecipeDir).
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	t.Cleanup(func() { config.RecipeDir = origRecipeDir })

	mux := NewRouter("")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health returned %d, want 200", resp.StatusCode)
	}
}

func TestNewRouterCertEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	t.Cleanup(func() { config.RecipeDir = origRecipeDir })

	// GetCertificate serves config.RootCA — create a dummy file.
	caFile := filepath.Join(tmpDir, "ca.crt")
	if err := os.WriteFile(caFile, []byte("FAKE-CA-CERT"), 0o644); err != nil {
		t.Fatal(err)
	}
	origRootCA := config.RootCA
	config.RootCA = caFile
	t.Cleanup(func() { config.RootCA = origRootCA })

	mux := NewRouter("")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/cert/")
	if err != nil {
		t.Fatalf("GET /auth/cert/: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /auth/cert/ returned %d, want 200", resp.StatusCode)
	}
}

func TestNewRouterFilesEndpointRequiresAuth(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	t.Cleanup(func() { config.RecipeDir = origRecipeDir })

	// Create a file in the recipe directory to serve.
	testFile := filepath.Join(tmpDir, "recipe.sls")
	if err := os.WriteFile(testFile, []byte("test recipe"), 0o644); err != nil {
		t.Fatal(err)
	}

	mux := NewRouter("")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Request without auth token should be rejected.
	resp, err := http.Get(srv.URL + "/files/recipe.sls")
	if err != nil {
		t.Fatalf("GET /files/recipe.sls: %v", err)
	}
	defer resp.Body.Close()
	// Without a valid token, expect 401 (Unauthorized).
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /files/ without auth returned %d, want 401", resp.StatusCode)
	}
}

func TestNewRouterMethodNotAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	t.Cleanup(func() { config.RecipeDir = origRecipeDir })

	mux := NewRouter("")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// POST to /health should not match the GET-only route.
	resp, err := http.Post(srv.URL+"/health", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /health: %v", err)
	}
	defer resp.Body.Close()
	// Go 1.22+ ServeMux returns 405 for wrong method on exact route.
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /health returned %d, want 405", resp.StatusCode)
	}
}

func TestNewRouterPutNKey(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	t.Cleanup(func() { config.RecipeDir = origRecipeDir })

	mux := NewRouter("")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// PUT /pki/putnkey with no body should return 400 (bad request)
	// because the handler tries to decode JSON and fails.
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/pki/putnkey", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /pki/putnkey: %v", err)
	}
	defer resp.Body.Close()
	// Handler tries to decode JSON from nil body -> 400 Bad Request.
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("PUT /pki/putnkey (empty body) returned %d, want 400", resp.StatusCode)
	}
}

func TestNewRouterUnknownPathReturns404(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	t.Cleanup(func() { config.RecipeDir = origRecipeDir })

	mux := NewRouter("")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /nonexistent returned %d, want 404", resp.StatusCode)
	}
}
