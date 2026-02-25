package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestDirectory(t *testing.T) {
	tempDir := t.TempDir()
	sampleDir := filepath.Join(tempDir, "there-is-a-dir-here")
	os.Mkdir(sampleDir, 0o755)
	file := filepath.Join(sampleDir, "there-is-a-file-here")
	os.Create(file)
	fileModeDNE := filepath.Join(sampleDir, "file-mode-does-not-exist")
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
			name: "DirectoryRoot",
			params: map[string]interface{}{
				"name": "/",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: ErrDeleteRoot,
		},
		{
			name: "DirectoryExistingNoAction",
			params: map[string]interface{}{
				"name": sampleDir,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     []fmt.Stringer{cook.Snprintf("directory %s already exists", sampleDir)},
			},
			error: nil,
		},
		{
			name: "DirectoryChangeMode",
			params: map[string]interface{}{
				"name":     sampleDir,
				"dir_mode": "755",
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     []fmt.Stringer{cook.Snprintf("directory %s already exists", sampleDir), cook.Snprintf("chmod %s to 755", sampleDir)},
			},
			error: nil,
		},
		{
			name: "DirectoryTestChangeDirMode",
			params: map[string]interface{}{
				"name":     sampleDir,
				"dir_mode": "755",
				"makedirs": true,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     []fmt.Stringer{cook.Snprintf("directory %s already exists", sampleDir), cook.Snprintf("would chmod %s to 755", sampleDir)},
			},
			test:  true,
			error: nil,
		},
		{
			name: "DirectoryChangeModeNotExist",
			params: map[string]interface{}{
				"name":     fileModeDNE,
				"dir_mode": "755",
				"makedirs": false,
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: fmt.Errorf("chmod %s: no such file or directory", fileModeDNE),
		},
		{
			name: "DirectoryTestChangeFileMode",
			params: map[string]interface{}{
				"name":      sampleDir,
				"file_mode": "755",
				"makedirs":  true,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Notes:     []fmt.Stringer{cook.Snprintf("directory %s already exists", sampleDir), cook.Snprintf("would chmod %s to 755", sampleDir)},
			},
			test:  true,
			error: nil,
		},
		{
			// TODO: Update to match error for a directory that doesn't exist
			name: "DirectoryChangeFileModeNotExist",
			params: map[string]interface{}{
				"name":      fileModeDNE,
				"file_mode": "755",
				"makedirs":  false,
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: fmt.Errorf("chmod %s: no such file or directory", fileModeDNE),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, err := f.directory(context.Background(), test.test)
			if err != nil || test.error != nil {
				if (err == nil && test.error != nil) || (err != nil && test.error == nil) {
					t.Errorf("expected error `%v`, got `%v`", test.error, err)
				} else if err.Error() != test.error.Error() {
					t.Errorf("expected error %v, got %v", test.error, err)
				}
			}
			compareResults(t, result, test.expected)
		})
	}
}
