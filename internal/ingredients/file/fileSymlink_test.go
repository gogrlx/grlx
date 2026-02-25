package file

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestSymlink(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := tempDir + "/test"
	tempTarget := tempDir + "/target"
	existingSymlink := tempDir + "/existingSymlink"
	os.Symlink(tempTarget, existingSymlink)
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
			name: "SymlinkRoot",
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
			name: "SymlinkMissingTarget",
			params: map[string]interface{}{
				"name": "/tmp/test",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: ErrMissingTarget,
		},
		{
			name: "SymlinkCreateTest",
			params: map[string]interface{}{
				"name":   tempFile,
				"target": tempTarget,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{cook.Snprintf("would create symlink %s pointing to %s", tempFile, tempTarget)},
			},
			error: nil,
			test:  true,
		},
		{
			name: "SymlinkCreate",
			params: map[string]interface{}{
				"name":   tempFile,
				"target": tempTarget,
			},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{cook.Snprintf("created symlink %s pointing to %s", tempFile, tempTarget)},
			},
			error: nil,
		},
		{
			name: "SymlinkExistingCreate",
			params: map[string]interface{}{
				"name":   existingSymlink,
				"target": tempTarget,
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "symlink",
				params: test.params,
			}
			result, err := f.symlink(context.TODO(), test.test)
			if test.error != nil && err.Error() != test.error.Error() {
				t.Errorf("expected error %v, got %v", test.error, err)
			}
			compareResults(t, result, test.expected)
		})
	}
}
