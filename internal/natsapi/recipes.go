package natsapi

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/config"
)

// RecipeInfo represents a recipe file in the listing.
type RecipeInfo struct {
	// Name is the dot-notation recipe name (e.g., "webserver.nginx").
	Name string `json:"name"`
	// Path is the relative file path from the recipe root.
	Path string `json:"path"`
	// Size is the file size in bytes.
	Size int64 `json:"size"`
}

// RecipeContent represents the full content of a recipe file.
type RecipeContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

func handleRecipesList(_ json.RawMessage) (any, error) {
	recipeDir := config.RecipeDir
	if recipeDir == "" {
		return nil, fmt.Errorf("recipe directory not configured")
	}

	info, err := os.Stat(recipeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]RecipeInfo{"recipes": {}}, nil
		}
		return nil, fmt.Errorf("cannot access recipe directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("recipe path is not a directory: %s", recipeDir)
	}

	var recipes []RecipeInfo
	ext := "." + config.GrlxExt

	err = filepath.WalkDir(recipeDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ext) {
			return nil
		}

		relPath, relErr := filepath.Rel(recipeDir, path)
		if relErr != nil {
			return nil
		}

		// Convert file path to dot-notation recipe name:
		// "webserver/nginx.grlx" -> "webserver.nginx"
		name := strings.TrimSuffix(relPath, ext)
		name = strings.ReplaceAll(name, string(filepath.Separator), ".")

		fi, statErr := d.Info()
		if statErr != nil {
			return nil
		}

		recipes = append(recipes, RecipeInfo{
			Name: name,
			Path: relPath,
			Size: fi.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking recipe directory: %w", err)
	}

	if recipes == nil {
		recipes = []RecipeInfo{}
	}
	return map[string][]RecipeInfo{"recipes": recipes}, nil
}

func handleRecipesGet(params json.RawMessage) (any, error) {
	var req struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	recipeName := req.Name
	if recipeName == "" {
		recipeName = req.ID
	}
	if recipeName == "" {
		return nil, fmt.Errorf("recipe name is required")
	}

	recipeDir := config.RecipeDir
	if recipeDir == "" {
		return nil, fmt.Errorf("recipe directory not configured")
	}

	// Convert dot-notation to file path and resolve
	// Use the same resolution logic as the cook system
	relPath := strings.ReplaceAll(recipeName, ".", string(filepath.Separator)) + "." + config.GrlxExt
	fullPath := filepath.Join(recipeDir, relPath)
	fullPath = filepath.Clean(fullPath)

	// Security: ensure the resolved path is within the recipe directory
	absRecipeDir, err := filepath.Abs(recipeDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve recipe directory: %w", err)
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve recipe path: %w", err)
	}
	if !strings.HasPrefix(absFullPath, absRecipeDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid recipe name: path traversal detected")
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("recipe not found: %s", recipeName)
		}
		return nil, fmt.Errorf("cannot access recipe: %w", err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("recipe path is a directory: %s", recipeName)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read recipe: %w", err)
	}

	return RecipeContent{
		Name:    recipeName,
		Path:    relPath,
		Content: string(content),
		Size:    fi.Size(),
	}, nil
}
