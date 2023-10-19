package file

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestMissing(t *testing.T) {
	tempDir := t.TempDir()
	fileDNE := filepath.Join(tempDir, "file-does-not-exist")
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
			name:   "FileMissing",
			params: map[string]interface{}{"name": fileDNE},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("file `%s` is missing", fileDNE)},
			},
			error: nil,
		},
		{
			name:   "FileExists",
			params: map[string]interface{}{"name": tempDir},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("file `%s` is not missing", tempDir)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, err := f.missing(context.TODO(), test.test)
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
