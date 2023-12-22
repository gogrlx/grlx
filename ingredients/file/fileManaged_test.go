package file

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestManaged(t *testing.T) {
	// TODO: Determine how to set up the farmer local file system path
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "managed-file")
	f, err := os.Create(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString("This is the existing file content")
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	existingFileHash := h.Sum(nil)
	hashString := fmt.Sprintf("md5:%x", existingFileHash)
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
			name: "Simple case",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      "grlx://test/managed-file",
				"skip_verify": true,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				// TODO: Notes not implemented yet for file.managed
				Notes: []fmt.Stringer{},
			},
			error: nil,
			test:  false,
		},
		{
			name: "Simple case with backup",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      "grlx://test/managed-file",
				"skip_verify": true,
				"backup":      true,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				// TODO: Notes not implemented yet for file.managed
				Notes: []fmt.Stringer{},
			},
			error: nil,
			test:  false,
		},
		{
			name: "Simple case with source_hash",
			params: map[string]interface{}{
				"name":        existingFile,
				"source":      "grlx://test/managed-file",
				"source_hash": hashString,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				// TODO: Notes not implemented yet for file.managed
				Notes: []fmt.Stringer{},
			},
			error: nil,
			test:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "managed",
				params: test.params,
			}
			result, err := f.managed(context.TODO(), test.test)
			if test.error != nil && err.Error() != test.error.Error() {
				t.Errorf("expected error %v, got %v", test.error, err)
			}
			compareResults(t, result, test.expected)
		})
	}
}
