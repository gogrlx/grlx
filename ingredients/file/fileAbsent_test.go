package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func compareResults(t *testing.T, result types.Result, expected types.Result) {
	if result.Succeeded != expected.Succeeded {
		t.Errorf("expected succeeded to be %v, got %v", expected.Succeeded, result.Succeeded)
	}
	if result.Failed != expected.Failed {
		t.Errorf("expected failed to be %v, got %v", expected.Failed, result.Failed)
	}
	if len(result.Notes) != len(expected.Notes) {
		t.Errorf("expected %v notes, got %v.\nGot %v", len(expected.Notes), len(result.Notes), result.Notes)
	}
	for i, note := range result.Notes {
		if note.String() != expected.Notes[i].String() {
			t.Errorf("expected note %v to be %v, got %v", i, expected.Notes[i], note)
		}
	}
}

func TestAbsent(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "there-is-a-file-here")
	os.Create(existingFile)
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
			name: "AbsentRoot",
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
			name: "AbsentNonExistent",
			params: map[string]interface{}{
				"name": filepath.Join(tempDir, "there-isnt-a-file-here"),
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("%s is already absent", filepath.Join(tempDir, "there-isnt-a-file-here"))},
			},
			error: nil,
		},
		{
			name: "AbsentTestRun",
			params: map[string]interface{}{
				"name": existingFile,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("%s would be deleted", existingFile)},
			},
			test: true,
		},
		{
			name: "AbsentTestActual",
			params: map[string]interface{}{
				"name": existingFile,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("%s has been deleted", existingFile)},
			},
		},
		{
			name: "AbesentDeletePopulatedDirs",
			params: map[string]interface{}{
				"name": sampleDir,
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: &os.PathError{Op: "remove", Path: sampleDir, Err: fmt.Errorf("directory not empty")},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, err := f.absent(context.TODO(), test.test)
			if test.error != nil && err.Error() != test.error.Error() {
				t.Errorf("expected error %v, got %v", test.error, err)
			}
			compareResults(t, result, test.expected)
		})
	}
}
