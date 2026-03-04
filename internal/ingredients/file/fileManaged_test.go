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
	tempDir := t.TempDir()
	cd := config.CacheDir
	defer func() { config.CacheDir = cd }()
	config.CacheDir = filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(config.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a source file for local provider tests
	sourceFile := filepath.Join(tempDir, "source-file")
	if err := os.WriteFile(sourceFile, []byte("source content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute hash of source file
	h := md5.New()
	h.Write([]byte("source content"))
	hashString := fmt.Sprintf("md5:%x", h.Sum(nil))

	existingFile := filepath.Join(tempDir, "managed-file")
	if err := os.WriteFile(existingFile, []byte("existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

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
		params   map[string]interface{}
		expected cook.Result
		error    error
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
				Notes:     []fmt.Stringer{},
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
				Notes:     []fmt.Stringer{},
			},
			error: ErrModifyRoot,
		},
		{
			name: "Simple case",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      sourceFile,
				"skip_verify": true,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
			},
			error: nil,
			test:  false,
		},
		{
			name: "Simple case with backup",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      sourceFile,
				"skip_verify": true,
				"backup":      true,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
			},
			error: nil,
			test:  false,
		},
		{
			name: "Simple case with source_hash",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      sourceFile,
				"source_hash": hashString,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
			},
			error: nil,
			test:  false,
		},
		{
			// TODO: Verify that this is the expected behavior
			name: "Simple case no create",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      sourceFile,
				"source_hash": hashString,
				"create":      false,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
			},
			error: nil,
			test:  false,
		},
		{
			name: "External case via HTTP",
			params: map[string]interface{}{
				"name":        filepath.Join(tempDir, "http-managed"),
				"source":      httpSource,
				"source_hash": hashString,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
			},
			error: nil,
			test:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     test.name,
				method: "managed",
				params: test.params,
			}
			result, err := f.managed(context.TODO(), test.test)
			if test.error != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", test.error)
				} else if err.Error() != test.error.Error() {
					t.Errorf("expected error %v, got %v", test.error, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if result.Succeeded != test.expected.Succeeded {
				t.Errorf("expected succeeded to be %v, got %v", test.expected.Succeeded, result.Succeeded)
			}
			if result.Failed != test.expected.Failed {
				t.Errorf("expected failed to be %v, got %v", test.expected.Failed, result.Failed)
			}
		})
	}
}
