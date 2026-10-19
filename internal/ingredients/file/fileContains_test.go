package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestContains(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "there-is-a-file-here")
	f, _ := os.Create(existingFile)
	defer f.Close()
	if _, err := f.WriteString("hello world"); err != nil {
		t.Fatal(err)
	}
	existingFileSrc := filepath.Join(tempDir, "there-is-a-src-here")
	f, _ = os.Create(existingFileSrc)
	defer f.Close()
	if _, err := f.WriteString("hello world"); err != nil {
		t.Fatal(err)
	}

	config.CacheDir = tempDir
	defer func() { config.CacheDir = "" }()

	tests := []struct {
		name     string
		params   map[string]interface{}
		expected cook.Result
		error    error
		test     bool
	}{
		{
			name: "IncorrectFilename",
			params: map[string]interface{}{
				"name": 1,
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: ingredients.ErrMissingName,
		},
		{
			name: "ContainsRoot",
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, _, err := f.contains(context.TODO(), test.test)
			if test.error != nil && err.Error() != test.error.Error() {
				t.Errorf("expected error %v, got %v", test.error, err)
			}
			compareResults(t, result, test.expected)
		})
	}
}

func TestContainsLongLine(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "long-line")
	existing := strings.Repeat("a", 70*1024)
	missing := strings.Repeat("b", 70*1024)
	if err := os.WriteFile(target, []byte(existing), 0o644); err != nil {
		t.Fatalf("failed to write target: %v", err)
	}

	f := File{
		id:     "long-line",
		method: "contains",
		params: map[string]interface{}{
			"name": target,
			"text": missing,
		},
	}
	result, missingContent, err := f.contains(context.TODO(), false)
	if !errors.Is(err, ErrMissingContent) {
		t.Fatalf("expected ErrMissingContent, got %v", err)
	}
	if result.Succeeded || !result.Failed {
		t.Fatalf("expected failed missing-content result, got %+v", result)
	}
	if got := missingContent.String(); got != missing {
		t.Fatalf("expected missing long line to be preserved, got %d bytes", len(got))
	}
}
