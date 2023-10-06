package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestDirectory(t *testing.T) {
	tempDir := t.TempDir()
	sampleDir := filepath.Join(tempDir, "there-is-a-dir-here")
	os.Mkdir(sampleDir, 0o755)
	file := filepath.Join(sampleDir, "there-is-a-file-here")
	os.Create(file)
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
			name: "DirectoryRoot",
			params: map[string]interface{}{
				"name": "/",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     []fmt.Stringer{},
			},
			error: types.ErrDeleteRoot,
		},
		{
			name: "DirectoryExistingNoAction",
			params: map[string]interface{}{
				"name": sampleDir,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Notes:     nil,
			},
			error: fmt.Errorf("method  undefined"),
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
			if err.Error() != test.error.Error() {
				t.Errorf("expected error to be %v, got %v", test.error, err)
			}
			compareResults(t, result, test.expected)
		})
	}
}
