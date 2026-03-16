package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestFileContent(t *testing.T) {
	tempDir := t.TempDir()
	cd := config.CacheDir
	defer func() { config.CacheDir = cd }()
	config.CacheDir = filepath.Join(tempDir, "cache")
	os.MkdirAll(config.CacheDir, 0o755)

	doesExist := filepath.Join(tempDir, "doesExist")
	if err := os.WriteFile(doesExist, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	sourceExist := filepath.Join(tempDir, "sourceExist")
	if err := os.WriteFile(sourceExist, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		params   map[string]interface{}
		expected cook.Result
		error    error
		errorIs  error
		test     bool
	}{
		{
			name: "incorrect name",
			params: map[string]interface{}{
				"name": 1,
				"text": "string",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
			},
			error: ingredients.ErrMissingName,
		},
		{
			name: "empty name",
			params: map[string]interface{}{
				"name": "",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
			},
			error: ingredients.ErrMissingName,
		},
		{
			name: "root",
			params: map[string]interface{}{
				"name": "/",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
			},
			error: ErrModifyRoot,
		},
		{
			name: "source missing hash",
			params: map[string]interface{}{
				"name":   doesExist,
				"source": "nope",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
			},
			error: ErrMissingHash,
		},
		{
			name: "sources missing hashes",
			params: map[string]interface{}{
				"name":          filepath.Join(tempDir, "test"),
				"sources":       []interface{}{sourceExist, doesExist},
				"source_hashes": []interface{}{"thing1"},
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes: []fmt.Stringer{
					cook.SimpleNote("sources and source_hashes must be the same length"),
				},
			},
			error: ErrMissingHash,
		},
		{
			name: "parent dir missing no makedirs",
			params: map[string]interface{}{
				"name": filepath.Join(tempDir, "nonexistent", "subdir", "file"),
				"text": "hello",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes: []fmt.Stringer{
					cook.Snprintf("parent directory `%s` does not exist and makedirs is false",
						filepath.Join(tempDir, "nonexistent", "subdir")),
				},
			},
			error: ErrPathNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "content",
				params: tc.params,
			}
			result, err := f.content(context.TODO(), tc.test)
			if err != nil {
				if tc.errorIs != nil {
					if !errors.Is(err, tc.errorIs) {
						t.Errorf("expected error to wrap %v, got %v", tc.errorIs, err)
					}
				} else if tc.error == nil {
					t.Errorf("expected error to be nil but got %v", err)
				} else if err.Error() != tc.error.Error() {
					t.Errorf("expected error %v, got %v", tc.error, err)
				}
			} else if tc.error != nil {
				t.Errorf("expected error %v, got nil", tc.error)
			}
			compareResults(t, result, tc.expected)
		})
	}
}

func TestFileContentTextWrite(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("write text to new file", func(t *testing.T) {
		target := filepath.Join(tempDir, "newfile.txt")
		f := File{
			id:     "test-write",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": "hello world",
			},
		}
		result, err := f.content(context.TODO(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if !result.Changed {
			t.Error("expected changed")
		}
		got, _ := os.ReadFile(target)
		if string(got) != "hello world\n" {
			t.Errorf("expected 'hello world\\n', got %q", string(got))
		}
	})

	t.Run("idempotent text write", func(t *testing.T) {
		target := filepath.Join(tempDir, "idempotent.txt")
		os.WriteFile(target, []byte("same content\n"), 0o644)
		f := File{
			id:     "test-idempotent",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": "same content",
			},
		}
		result, err := f.content(context.TODO(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if result.Changed {
			t.Error("expected no change for idempotent write")
		}
	})

	t.Run("update existing file", func(t *testing.T) {
		target := filepath.Join(tempDir, "update.txt")
		os.WriteFile(target, []byte("old content\n"), 0o644)
		f := File{
			id:     "test-update",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": "new content",
			},
		}
		result, err := f.content(context.TODO(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if !result.Changed {
			t.Error("expected changed")
		}
		got, _ := os.ReadFile(target)
		if string(got) != "new content\n" {
			t.Errorf("expected 'new content\\n', got %q", string(got))
		}
	})

	t.Run("text as slice", func(t *testing.T) {
		target := filepath.Join(tempDir, "slice.txt")
		f := File{
			id:     "test-slice",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": []interface{}{"line1", "line2", "line3"},
			},
		}
		result, err := f.content(context.TODO(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		got, _ := os.ReadFile(target)
		expected := "line1\nline2\nline3\n"
		if string(got) != expected {
			t.Errorf("expected %q, got %q", expected, string(got))
		}
	})

	t.Run("empty content to empty file is idempotent", func(t *testing.T) {
		target := filepath.Join(tempDir, "empty.txt")
		os.WriteFile(target, []byte{}, 0o644)
		f := File{
			id:     "test-empty",
			method: "content",
			params: map[string]interface{}{
				"name": target,
			},
		}
		result, err := f.content(context.TODO(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if result.Changed {
			t.Error("expected no change")
		}
	})
}

func TestFileContentMakedirs(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("makedirs creates parent and writes file", func(t *testing.T) {
		target := filepath.Join(tempDir, "deep", "nested", "dir", "file.txt")
		f := File{
			id:     "test-makedirs",
			method: "content",
			params: map[string]interface{}{
				"name":     target,
				"text":     "nested content",
				"makedirs": true,
			},
		}
		result, err := f.content(context.TODO(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Errorf("expected succeeded, got %+v", result)
		}
		if !result.Changed {
			t.Error("expected changed")
		}
		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("file not created: %v", readErr)
		}
		if string(got) != "nested content\n" {
			t.Errorf("expected 'nested content\\n', got %q", string(got))
		}
	})

	t.Run("test mode with makedirs", func(t *testing.T) {
		target := filepath.Join(tempDir, "testmode", "sub", "file.txt")
		f := File{
			id:     "test-makedirs-test",
			method: "content",
			params: map[string]interface{}{
				"name":     target,
				"text":     "test content",
				"makedirs": true,
			},
		}
		result, err := f.content(context.TODO(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if !result.Changed {
			t.Error("expected changed in test mode")
		}
		// File should NOT actually be created in test mode.
		if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
			t.Error("file should not exist in test mode")
		}
	})
}

func TestFileContentTestMode(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("test mode reports would create", func(t *testing.T) {
		target := filepath.Join(tempDir, "wouldcreate.txt")
		f := File{
			id:     "test-mode-create",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": "hello",
			},
		}
		result, err := f.content(context.TODO(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if !result.Changed {
			t.Error("expected changed")
		}
		if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
			t.Error("file should not be created in test mode")
		}
	})

	t.Run("test mode reports would update", func(t *testing.T) {
		target := filepath.Join(tempDir, "wouldupdate.txt")
		os.WriteFile(target, []byte("old\n"), 0o644)
		f := File{
			id:     "test-mode-update",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": "new",
			},
		}
		result, err := f.content(context.TODO(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if !result.Changed {
			t.Error("expected changed")
		}
		// File should still have old content.
		got, _ := os.ReadFile(target)
		if string(got) != "old\n" {
			t.Errorf("file should be unchanged in test mode, got %q", string(got))
		}
	})

	t.Run("test mode no change when content matches", func(t *testing.T) {
		target := filepath.Join(tempDir, "nochange.txt")
		os.WriteFile(target, []byte("matching\n"), 0o644)
		f := File{
			id:     "test-mode-nochange",
			method: "content",
			params: map[string]interface{}{
				"name": target,
				"text": "matching",
			},
		}
		result, err := f.content(context.TODO(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded {
			t.Error("expected succeeded")
		}
		if result.Changed {
			t.Error("expected no change when content matches")
		}
	})
}

func init() {
	// Ensure notes comparison doesn't fail on nil vs empty slice.
	_ = fmt.Stringer(cook.SimpleNote(""))
}
