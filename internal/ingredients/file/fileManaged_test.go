package file

import (
	"context"
	"crypto/md5"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestManaged(t *testing.T) {
	// Create a source file for local provider tests
	sourceDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "source-file")
	if err := os.WriteFile(sourceFile, []byte("source content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute hash of source file
	h := md5.New()
	h.Write([]byte("source content"))
	hashString := fmt.Sprintf("md5:%x", h.Sum(nil))

	// Set up a local HTTP server for the external case
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("source content"))
	}))
	port := listener.Addr().(*net.TCPAddr).Port
	httpSource := fmt.Sprintf("http://localhost:%d/managed-file", port)

	tests := []struct {
		name     string
		params   func(tempDir string) map[string]interface{}
		expected func(tempDir, cacheDir string) cook.Result
		error    error
		test     bool
	}{
		{
			name: "incorrect name",
			params: func(_ string) map[string]interface{} {
				return map[string]interface{}{
					"name": 1,
					"text": "string",
				}
			},
			expected: func(_, _ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ingredients.ErrMissingName,
		},
		{
			name: "root",
			params: func(_ string) map[string]interface{} {
				return map[string]interface{}{
					"name": "/",
				}
			},
			expected: func(_, _ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ErrModifyRoot,
		},
		{
			name: "missing source",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":        filepath.Join(tempDir, "managed-file"),
					"skip_verify": true,
				}
			},
			expected: func(_, _ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ErrMissingSource,
		},
		{
			name: "missing hash without skip_verify",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":   filepath.Join(tempDir, "managed-file"),
					"source": sourceFile,
				}
			},
			expected: func(_, _ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes:     []fmt.Stringer{},
				}
			},
			error: ErrMissingHash,
		},
		{
			name: "Simple case",
			params: func(tempDir string) map[string]interface{} {
				existingFile := filepath.Join(tempDir, "managed-file")
				os.WriteFile(existingFile, []byte("existing content"), 0o644)
				return map[string]interface{}{
					"name":        existingFile,
					"source":      sourceFile,
					"skip_verify": true,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("%s has been cached", filepath.Join(cacheDir, "skip_Simple case-source")),
						cook.Snprintf("file `%s` managed from source `%s`", filepath.Join(tempDir, "managed-file"), sourceFile),
					},
				}
			},
			error: nil,
			test:  false,
		},
		{
			name: "Simple case with backup",
			params: func(tempDir string) map[string]interface{} {
				existingFile := filepath.Join(tempDir, "managed-file")
				os.WriteFile(existingFile, []byte("existing content"), 0o644)
				return map[string]interface{}{
					"name":        existingFile,
					"source":      sourceFile,
					"skip_verify": true,
					"backup":      true,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("%s has been cached", filepath.Join(cacheDir, "skip_Simple case with backup-source")),
						cook.Snprintf("file `%s` managed from source `%s`", filepath.Join(tempDir, "managed-file"), sourceFile),
					},
				}
			},
			error: nil,
			test:  false,
		},
		{
			name: "Simple case with source_hash",
			params: func(tempDir string) map[string]interface{} {
				existingFile := filepath.Join(tempDir, "managed-file")
				os.WriteFile(existingFile, []byte("existing content"), 0o644)
				return map[string]interface{}{
					"name":        existingFile,
					"source":      sourceFile,
					"source_hash": hashString,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("%s has been cached", filepath.Join(cacheDir, hashString)),
						cook.Snprintf("file `%s` managed from source `%s`", filepath.Join(tempDir, "managed-file"), sourceFile),
					},
				}
			},
			error: nil,
			test:  false,
		},
		{
			// When create=false and the file exists, it should still be managed
			name: "Simple case no create",
			params: func(tempDir string) map[string]interface{} {
				existingFile := filepath.Join(tempDir, "managed-file")
				os.WriteFile(existingFile, []byte("existing content"), 0o644)
				return map[string]interface{}{
					"name":        existingFile,
					"source":      sourceFile,
					"source_hash": hashString,
					"create":      false,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("%s has been cached", filepath.Join(cacheDir, hashString)),
						cook.Snprintf("file `%s` managed from source `%s`", filepath.Join(tempDir, "managed-file"), sourceFile),
					},
				}
			},
			error: nil,
			test:  false,
		},
		{
			name: "no create file missing",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":        filepath.Join(tempDir, "nonexistent"),
					"source":      sourceFile,
					"source_hash": hashString,
					"create":      false,
				}
			},
			expected: func(tempDir, _ string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   false,
					Notes: []fmt.Stringer{
						cook.Snprintf("file `%s` does not exist and create is false", filepath.Join(tempDir, "nonexistent")),
					},
				}
			},
			error: nil,
			test:  false,
		},
		{
			name: "makedirs",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":        filepath.Join(tempDir, "subdir", "managed-file"),
					"source":      sourceFile,
					"skip_verify": true,
					"makedirs":    true,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("created directory `%s`", filepath.Join(tempDir, "subdir")),
						cook.Snprintf("%s has been cached", filepath.Join(cacheDir, "skip_makedirs-source")),
						cook.Snprintf("file `%s` managed from source `%s`", filepath.Join(tempDir, "subdir", "managed-file"), sourceFile),
					},
				}
			},
			error: nil,
			test:  false,
		},
		{
			name: "makedirs test mode",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":        filepath.Join(tempDir, "newdir", "managed-file"),
					"source":      sourceFile,
					"skip_verify": true,
					"makedirs":    true,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("directory `%s` would be created", filepath.Join(tempDir, "newdir")),
						cook.Snprintf("%s would be cached", filepath.Join(cacheDir, "skip_makedirs test mode-source")),
					},
				}
			},
			error: nil,
			test:  true,
		},
		{
			name: "parent dir missing no makedirs",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":        filepath.Join(tempDir, "nodir", "managed-file"),
					"source":      sourceFile,
					"skip_verify": true,
				}
			},
			expected: func(tempDir, _ string) cook.Result {
				return cook.Result{
					Succeeded: false,
					Failed:    true,
					Notes: []fmt.Stringer{
						cook.Snprintf("parent directory `%s` does not exist and makedirs is false", filepath.Join(tempDir, "nodir")),
					},
				}
			},
			error: ErrPathNotFound,
			test:  false,
		},
		{
			name: "External case via HTTP",
			params: func(tempDir string) map[string]interface{} {
				return map[string]interface{}{
					"name":        filepath.Join(tempDir, "http-managed"),
					"source":      httpSource,
					"source_hash": hashString,
				}
			},
			expected: func(tempDir, cacheDir string) cook.Result {
				return cook.Result{
					Succeeded: true,
					Failed:    false,
					Changed:   true,
					Notes: []fmt.Stringer{
						cook.Snprintf("%s has been cached", filepath.Join(cacheDir, hashString)),
						cook.Snprintf("file `%s` managed from source `%s`", filepath.Join(tempDir, "http-managed"), httpSource),
					},
				}
			},
			error: nil,
			test:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			cacheDir := filepath.Join(tempDir, "cache")
			if err := os.MkdirAll(cacheDir, 0o755); err != nil {
				t.Fatal(err)
			}
			origCache := config.CacheDir
			config.CacheDir = cacheDir
			defer func() { config.CacheDir = origCache }()

			params := tc.params(tempDir)
			f := File{
				id:     tc.name,
				method: "managed",
				params: params,
			}
			result, err := f.managed(context.TODO(), tc.test)
			if tc.error != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tc.error)
				} else if err.Error() != tc.error.Error() {
					t.Errorf("expected error %v, got %v", tc.error, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			compareResults(t, result, tc.expected(tempDir, cacheDir))
		})
	}
}
