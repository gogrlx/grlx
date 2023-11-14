package file

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gogrlx/grlx/types"
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
		expected types.Result
		error    error
		test     bool
	}{
		{
			name: "IncorrectFilename",
			params: map[string]interface{}{
				"name": 1,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrMissingName,
		},
		{
			name: "SymlinkRoot",
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
			name: "SymlinkMissingTarget",
			params: map[string]interface{}{
				"name": "/tmp/test",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrMissingTarget,
		},
		{
			name: "SymlinkCreateTest",
			params: map[string]interface{}{
				"name":   tempFile,
				"target": tempTarget,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("would create symlink %s pointing to %s", tempFile, tempTarget)},
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
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("created symlink %s pointing to %s", tempFile, tempTarget)},
			},
			error: nil,
		},
		{
			name: "SymlinkExistingCreate",
			params: map[string]interface{}{
				"name":   existingSymlink,
				"target": tempTarget,
			},
			expected: types.Result{
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
