package file

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/ingredients"
)

func TestMissing(t *testing.T) {
	tempDir := t.TempDir()
	fileDNE := filepath.Join(tempDir, "file-does-not-exist")
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
			name:   "FileMissing",
			params: map[string]interface{}{"name": fileDNE},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{cook.Snprintf("file `%s` is missing", fileDNE)},
			},
			error: nil,
		},
		{
			name:   "FileExists",
			params: map[string]interface{}{"name": tempDir},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{cook.Snprintf("file `%s` is not missing", tempDir)},
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
