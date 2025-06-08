package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/config"
	"github.com/gogrlx/grlx/v2/types"
)

func TestFileContent(t *testing.T) {
	tempDir := t.TempDir()
	cd := config.CacheDir
	// Restore config.CacheDir after test
	defer func() { config.CacheDir = cd }()
	config.CacheDir = filepath.Join(tempDir, "cache")
	newDir := filepath.Join(tempDir, "this/item")
	dirEntry := filepath.Dir(newDir)
	fmt.Println(newDir)
	doesExist := filepath.Join(tempDir, "doesExist")
	_, err := os.Create(doesExist)
	if err != nil {
		t.Fatal(err)
	}
	sourceExist := filepath.Join(tempDir, "sourceExist")
	_, err = os.Create(sourceExist)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected types.Result
		error    error
		test     bool
	}{
		{
			name: "incorrect name",
			params: map[string]interface{}{
				"name": 1,
				"text": "string",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrMissingName,
		},
		{
			name: "root",
			params: map[string]interface{}{
				"name": "/",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrModifyRoot,
		},
		{
			name: "makedirs",
			params: map[string]interface{}{
				"name":     newDir,
				"makedirs": true,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   true,
				Notes: []fmt.Stringer{
					types.Snprintf("created directory %s", dirEntry),
				},
			},
			error: nil,
			test:  false,
		},
		{
			name: "skip_verify file exists",
			params: map[string]interface{}{
				"name":        doesExist,
				"skip_verify": true,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: nil,
			test:  false,
		},
		{
			name: "source missing hash",
			params: map[string]interface{}{
				"name":   doesExist,
				"source": "nope",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrMissingHash,
			test:  false,
		},
		{
			name: "sources missing hashes",
			params: map[string]interface{}{
				"name":          "test",
				"sources":       []string{sourceExist, doesExist},
				"source_hashes": []string{"thing1"},
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes: []fmt.Stringer{
					types.Snprintf("sources and source_hashes must be the same length"),
				},
			},
			error: types.ErrMissingHash,
			test:  false,
		},
		// Expect this to match the single source case
		{
			name: "sources missing hashes w/ skip_verify",
			params: map[string]interface{}{
				"name":        "test",
				"sources":     []string{sourceExist, doesExist},
				"skip_verify": true,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: nil,
			test:  false,
		},
		{
			name: "source with hash",
			params: map[string]interface{}{
				"name":        doesExist,
				"source":      sourceExist,
				"source_hash": "test1",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			// TODO: This should be a lot cleaner, relying on a stdblib error that we have little control over is difficult to test.
			error: errors.Join(fmt.Errorf("open %s: no such file or directory", filepath.Join(config.CacheDir, "test1")), types.ErrCacheFailure),
			test:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "content",
				params: test.params,
			}
			result, err := f.content(context.TODO(), test.test)
			if err != nil {
				if test.error == nil {
					t.Errorf("expected error to be nil but got %v", err)
				} else if err.Error() != test.error.Error() {
					t.Errorf("expected error %v, got %v", test.error, err)
				}
			}
			compareResults(t, result, test.expected)
		})
	}
}
