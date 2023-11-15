package file

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestTouch(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "there-is-a-file-here")
	missingBase := filepath.Join(tempDir, "there-isnt-a-dir-here")
	missingDir := filepath.Join(missingBase, "item")
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected types.Result
		error    error
		test     bool
	}{
		{
			name:   "IncorrrectFilename",
			params: map[string]interface{}{},
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
			name: "TouchRoot",
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
			name: "TouchFile",
			params: map[string]interface{}{
				"name": existingFile,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("timestamps of `%s` changed", existingFile)},
			},
			error: nil,
			test:  false,
		},
		{
			name: "TouchFileAtime",
			params: map[string]interface{}{
				"name":  existingFile,
				"atime": "2021-01-01T00:00:00Z",
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("timestamps of `%s` changed", existingFile)},
			},
			error: nil,
			test:  false,
		},
		{
			name: "TouchFileAtimeFail",
			params: map[string]interface{}{
				"name":  existingFile,
				"atime": "-1",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("failed to parse atime")},
			},
			error: nil,
			test:  false,
		},
		{
			name: "TouchFileMtime",
			params: map[string]interface{}{
				"name":  existingFile,
				"mtime": "2021-01-01T00:00:00Z",
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("timestamps of `%s` changed", existingFile)},
			},
			error: nil,
			test:  false,
		},
		{
			name: "TouchFileMtimeFail",
			params: map[string]interface{}{
				"name":  existingFile,
				"mtime": "-1",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("failed to parse mtime")},
			},
			error: nil,
			test:  false,
		},
		{
			name: "TouchFileMakeDirs",
			params: map[string]interface{}{
				"name":     existingFile,
				"makedirs": true,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("timestamps of `%s` changed", existingFile)},
			},
			error: nil,
			test:  false,
		},
		{
			name: "TouchFileMakeDirsMissingDir",
			params: map[string]interface{}{
				"name": missingDir,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("filepath `%s` is missing and `makedirs` is false", missingBase)},
			},
			error: types.ErrPathNotFound,
			test:  false,
		},
		{
			name: "TouchFileMakeDirsDir",
			params: map[string]interface{}{
				"name":     missingDir,
				"makedirs": true,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("file `%s` to be created with provided timestamps", missingDir)},
			},
			error: nil,
			test:  true,
		},
		// TODO: this is currently failing on the notes comparison
		{
			name: "TouchFileTestMTime",
			params: map[string]interface{}{
				"name":  existingFile,
				"mtime": "2021-01-01T00:00:00Z",
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("mtime of `%s` will be changed", missingDir)},
			},
			error: nil,
			test:  true,
		},
		// TODO: this is currently failing on the notes comparison
		{
			name: "TouchFileTestATime",
			params: map[string]interface{}{
				"name":  existingFile,
				"atime": "2021-01-01T00:00:00Z",
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("atime of `%s` will be changed", missingDir)},
			},
			error: nil,
			test:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "absent",
				params: test.params,
			}
			result, err := f.touch(context.TODO(), test.test)
			if test.error != nil && err.Error() != test.error.Error() {
				t.Errorf("expected error %v, got %v", test.error, err)
			}
			compareResults(t, result, test.expected)
		})
	}
}
