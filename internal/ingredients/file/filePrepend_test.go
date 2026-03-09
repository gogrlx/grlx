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

func TestPrepend(t *testing.T) {
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
	if err != nil {
		t.Fatalf("failed to write to file: %v", err)
	}
	f.Close()

	fileWithoutContent := filepath.Join(tempDir, "there-is-a-file-without-content-here")
	_, err = os.Create(fileWithoutContent)
	if err != nil {
		t.Fatalf("failed to create file without content %s: %v", fileWithoutContent, err)
	}

	fakePath := filepath.Join("/", "fakepath")

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
			name: "PrependRoot",
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
			name: "PrependFileInvalidPermissions",
			params: map[string]interface{}{
				"name": fileReadOnly,
				"text": "test",
			},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{cook.Snprintf("file %s does not contain all specified content", fileReadOnly)},
			},
			error: fmt.Errorf("failed to open %s for prepending: open %s: permission denied", fileReadOnly, fileReadOnly),
		},
		{
			name:   "PrependFileWithContent",
			params: map[string]interface{}{"name": fileWithContent, "text": "test"},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{},
			},
			error: nil,
		},
		{
			name:   "PrependFileWithoutContent",
			params: map[string]interface{}{"name": fileWithoutContent, "text": "test"},
			expected: cook.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   false,
				Notes:     []fmt.Stringer{cook.Snprintf("file %s does not contain all specified content", fileWithoutContent), cook.Snprintf("prepended %s", fileWithoutContent)},
			},
			error: nil,
		},
		{
			name:   "PrependDirectory",
			params: map[string]interface{}{"name": fakePath},
			expected: cook.Result{
				Succeeded: false,
				Failed:    true,
				Changed:   false,
				Notes:     []fmt.Stringer{cook.Snprintf("failed to open %s", fakePath)},
			},
			error: fmt.Errorf("failed to create %s: open %s: permission denied", fakePath, fakePath),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "",
				params: test.params,
			}
			result, err := f.prepend(context.TODO(), test.test)
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

func TestPrependOrder(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "prepend-order-test")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	_, err = f.WriteString("existing line\n")
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	f.Close()

	fi := File{
		id:     "",
		method: "",
		params: map[string]interface{}{
			"name": filePath,
			"text": "new line",
		},
	}
	result, err := fi.prepend(context.TODO(), false)
	if err != nil {
		t.Fatalf("prepend failed: %v", err)
	}
	if !result.Succeeded {
		t.Fatalf("prepend did not succeed")
	}
	if !result.Changed {
		t.Fatalf("prepend did not report changed")
	}

	contents, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := "new line\nexisting line\n"
	if string(contents) != expected {
		t.Errorf("expected %q, got %q", expected, string(contents))
	}
}

func TestPrependTestMode(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "prepend-test-mode")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	_, err = f.WriteString("original\n")
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	f.Close()

	fi := File{
		id:     "",
		method: "",
		params: map[string]interface{}{
			"name": filePath,
			"text": "new content",
		},
	}
	result, err := fi.prepend(context.TODO(), true)
	if err != nil {
		t.Fatalf("prepend test mode failed: %v", err)
	}
	if !result.Succeeded {
		t.Fatalf("prepend test mode did not succeed")
	}
	if !result.Changed {
		t.Fatalf("prepend test mode did not report changed")
	}

	// File should be unchanged in test mode.
	contents, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(contents) != "original\n" {
		t.Errorf("test mode modified the file: got %q", string(contents))
	}
}
