package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogrlx/grlx/types"
)

func TestAppend(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "there-is-a-file-here")
	os.Create(existingFile)
	sampleDir := filepath.Join(tempDir, "there-is-a-dir-here")
	os.Mkdir(sampleDir, 0o755)
	file := filepath.Join(sampleDir, "there-is-a-file-here")
	os.Create(file)
	fileReadOnly := filepath.Join(tempDir, "there-is-a-read-only-file-here")
	_, err := os.Create(fileReadOnly)
	if err != nil {
		t.Fatalf("failed to create read-only file %s: %v", fileReadOnly, err)
	}
	err = os.Chmod(fileReadOnly, 0o444)
	if err != nil {
		t.Fatalf("failed to chmod read-only file %s: %v", fileReadOnly, err)
	}

	fileWithContent := filepath.Join(tempDir, "there-is-a-file-with-content-here")
	f, err := os.Create(fileWithContent)
	if err != nil {
		t.Fatalf("failed to create file with content %s: %v", fileWithContent, err)
	}
	_, err = f.WriteString("test")
	f.Close()

	fileWithoutContent := filepath.Join(tempDir, "there-is-a-file-without-content-here")
	_, err = os.Create(fileWithoutContent)
	if err != nil {
		t.Fatalf("failed to create file without content %s: %v", fileWithoutContent, err)
	}

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
			name: "AppendRoot",
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
			name: "AppendFileInvalidPermissions",
			params: map[string]interface{}{
				"name": fileReadOnly,
				"text": "test",
			},
			expected: types.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("file %s does not contain all specified content", fileReadOnly)},
			},
			error: fmt.Errorf("open %s: permission denied", fileReadOnly),
		},
		{
			name:   "AppendFileWithContent",
			params: map[string]interface{}{"name": fileWithContent, "text": "test"},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: nil,
		},
		{
			name:   "AppendFileWithoutContent",
			params: map[string]interface{}{"name": fileWithoutContent, "text": "test"},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{types.Snprintf("file %s does not contain all specified content", fileWithoutContent), types.Snprintf("appended %s", fileWithoutContent)},
			},
			error: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, err := f.append(context.TODO(), test.test)
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
