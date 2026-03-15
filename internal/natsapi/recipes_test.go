package natsapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func TestHandleRecipesList(t *testing.T) {
	// Create a temporary recipe directory
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	// Create some test recipe files
	recipes := map[string]string{
		"webserver/nginx.grlx":        "steps:\n  install_nginx:\n    pkg.installed:\n      - name: nginx\n",
		"database/postgres.grlx":      "steps:\n  install_pg:\n    pkg.installed:\n      - name: postgresql\n",
		"webserver/nginx/vhosts.grlx": "steps:\n  copy_vhost:\n    file.managed:\n      - source: vhost.conf\n",
		"README.md":                   "This is not a recipe",
	}
	for relPath, content := range recipes {
		fullPath := filepath.Join(tmpDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := handleRecipesList(nil)
	if err != nil {
		t.Fatal(err)
	}

	data, ok := result.(map[string][]RecipeInfo)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	recipeList := data["recipes"]

	// Should find 3 .grlx files, not the README
	if len(recipeList) != 3 {
		t.Fatalf("expected 3 recipes, got %d: %+v", len(recipeList), recipeList)
	}

	// Check that all expected names are present
	nameSet := make(map[string]bool)
	for _, r := range recipeList {
		nameSet[r.Name] = true
	}

	expectedNames := []string{"webserver.nginx", "database.postgres", "webserver.nginx.vhosts"}
	for _, name := range expectedNames {
		if !nameSet[name] {
			t.Errorf("expected recipe %q not found in list: %v", name, nameSet)
		}
	}
}

func TestHandleRecipesGet(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	content := "steps:\n  install_nginx:\n    pkg.installed:\n      - name: nginx\n"
	recipePath := filepath.Join(tmpDir, "webserver", "nginx.grlx")
	if err := os.MkdirAll(filepath.Dir(recipePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recipePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	params, _ := json.Marshal(map[string]string{"name": "webserver.nginx"})

	result, err := handleRecipesGet(params)
	if err != nil {
		t.Fatal(err)
	}

	rc, ok := result.(RecipeContent)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	if rc.Name != "webserver.nginx" {
		t.Errorf("expected name %q, got %q", "webserver.nginx", rc.Name)
	}
	if rc.Content != content {
		t.Errorf("expected content %q, got %q", content, rc.Content)
	}
}

func TestHandleRecipesGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	params, _ := json.Marshal(map[string]string{"name": "nonexistent.recipe"})

	_, err := handleRecipesGet(params)
	if err == nil {
		t.Fatal("expected error for nonexistent recipe")
	}
}

func TestHandleRecipesGetPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	// Create a file outside the recipe dir
	outsidePath := filepath.Join(tmpDir, "..", "secret.grlx")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outsidePath)

	// Attempt path traversal via the name field
	params, _ := json.Marshal(map[string]string{"name": "..%2fsecret"})
	_, err := handleRecipesGet(params)
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
}

func TestHandleRecipesListEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	origRecipeDir := config.RecipeDir
	config.RecipeDir = tmpDir
	defer func() { config.RecipeDir = origRecipeDir }()

	result, err := handleRecipesList(nil)
	if err != nil {
		t.Fatal(err)
	}

	data, ok := result.(map[string][]RecipeInfo)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	if len(data["recipes"]) != 0 {
		t.Fatalf("expected empty list, got %d recipes", len(data["recipes"]))
	}
}

func TestHandleRecipesListNonexistentDir(t *testing.T) {
	origRecipeDir := config.RecipeDir
	config.RecipeDir = "/nonexistent/path/to/recipes"
	defer func() { config.RecipeDir = origRecipeDir }()

	result, err := handleRecipesList(nil)
	if err != nil {
		t.Fatal(err)
	}

	data, ok := result.(map[string][]RecipeInfo)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	if len(data["recipes"]) != 0 {
		t.Fatalf("expected empty list for nonexistent dir, got %d", len(data["recipes"]))
	}
}
