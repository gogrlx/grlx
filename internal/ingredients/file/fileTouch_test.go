package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/djherbis/atime"
	"github.com/gogrlx/grlx/v2/internal/types"
)

func TestTouch(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "there-is-a-file-here")
	_, err := os.Create(existingFile)
	if err != nil {
		t.Fatal(err)
	}
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
			name:   "no name",
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
			name: "root",
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
			name: "default",
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
			name: "default test",
			params: map[string]interface{}{
				"name": existingFile,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("timestamps of `%s` will be changed", existingFile)},
			},
			error: nil,
			test:  true,
		},
		{
			name: "atime",
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
			name: "failed atime",
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
			name: "change mtime",
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
			name: "improper mtime",
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
			name: "makedirs true",
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
			name: "missing dir makedirs false",
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
			name: "missing dir makedirs true test",
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
		{
			name: "missing dir makedirs true",
			params: map[string]interface{}{
				"name":     missingDir,
				"makedirs": true,
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("timestamps of `%s` changed", missingDir)},
			},
			error: nil,
			test:  false,
		},
		{
			name: "test mtime",
			params: map[string]interface{}{
				"name":  existingFile,
				"mtime": "2021-01-01T00:00:00Z",
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("mtime of `%s` will be changed", existingFile)},
			},
			error: nil,
			test:  true,
		},
		{
			name: "test atime",
			params: map[string]interface{}{
				"name":  existingFile,
				"atime": "2021-01-01T00:00:00Z",
			},
			expected: types.Result{
				Succeeded: true,
				Failed:    false,
				Changed:   true,
				Notes:     []fmt.Stringer{types.Snprintf("atime of `%s` will be changed", existingFile)},
			},
			error: nil,
			test:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := File{
				id:     "",
				method: "touch",
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

// Validates that the times are set properly when both are provided.
func TestTouchValidate(t *testing.T) {
	testDir := t.TempDir()
	existingFile := filepath.Join(testDir, "there-is-a-file-here")
	fileTest, err := os.Create(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	fileHandler, _ := fileTest.Stat()
	mt := fileHandler.ModTime()
	at, _ := atime.Stat(existingFile)
	baseTime := "2021-01-01T00:00:00Z"
	setMtime, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	setAtime, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	f := File{
		id:     "",
		method: "touch",
		params: map[string]interface{}{
			"name":  existingFile,
			"atime": baseTime,
			"mtime": baseTime,
		},
	}
	f.touch(context.TODO(), false)
	testHandler, _ := fileTest.Stat()
	tmt := testHandler.ModTime()
	tat, _ := atime.Stat(existingFile)
	if mt.Equal(tmt) {
		t.Error("mtime timestamps are equal when they shouldn't be")
	}
	if at.Equal(tat) {
		t.Error("atime timestamps are equal when they shouldn't be")
	}
	if !tmt.UTC().Equal(setMtime) {
		t.Errorf("expected mtime to be %v, got %v", setMtime, mt.UTC())
	}
	if !tat.UTC().Equal(setAtime) {
		t.Errorf("expected atime to be %v, got %v", setAtime, at.UTC())
	}
}

// Validates that the mtime is set properly when only the mtime is provided
// Also validates that the atime is not changed.
func TestOnlyMtime(t *testing.T) {
	testDir := t.TempDir()
	existingFile := filepath.Join(testDir, "there-is-a-file-here")
	fileTest, err := os.Create(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	fileHandler, _ := fileTest.Stat()
	mt := fileHandler.ModTime()
	at, _ := atime.Stat(existingFile)
	baseTime := "2021-01-01T00:00:00Z"
	setMtime, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	f := File{
		id:     "",
		method: "touch",
		params: map[string]interface{}{
			"name":  existingFile,
			"mtime": baseTime,
		},
	}
	f.touch(context.TODO(), false)
	testHandler, _ := fileTest.Stat()
	tmt := testHandler.ModTime()
	tat, _ := atime.Stat(existingFile)
	if mt.Equal(tmt) {
		t.Error("mtime timestamps are equal when they shouldn't be")
	}
	if !at.UTC().Equal(tat) {
		t.Errorf("expected atime to be %v, got %v", at.UTC(), tat.UTC())
	}
	if !tmt.UTC().Equal(setMtime) {
		t.Errorf("expected mtime to be %v, got %v", setMtime, mt.UTC())
	}
}

// Validates that the atime is set properly when only the atime is provided.
// Also validates that the mtime is not changed.
func TestOnlyAtime(t *testing.T) {
	testDir := t.TempDir()
	existingFile := filepath.Join(testDir, "there-is-a-file-here")
	fileTest, err := os.Create(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	fileHandler, _ := fileTest.Stat()
	mt := fileHandler.ModTime()
	at, _ := atime.Stat(existingFile)
	baseTime := "2021-01-01T00:00:00Z"
	setAtime, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	f := File{
		id:     "",
		method: "touch",
		params: map[string]interface{}{
			"name":  existingFile,
			"atime": baseTime,
		},
	}
	f.touch(context.TODO(), false)
	testHandler, _ := fileTest.Stat()
	tmt := testHandler.ModTime()
	tat, _ := atime.Stat(existingFile)
	if at.Equal(tat) {
		t.Error("atime timestamps are equal when they shouldn't be")
	}
	if !mt.Equal(tmt) {
		t.Errorf("expected mtime to be %v, got %v", mt.UTC(), tmt.UTC())
	}
	if !tat.UTC().Equal(setAtime) {
		t.Errorf("expected atime to be %v, got %v", setAtime, at.UTC())
	}
}

// Validates when a file already has the provided timestamps
func TestTouchAlreadySet(t *testing.T) {
	testDir := t.TempDir()
	existingFile := filepath.Join(testDir, "there-is-a-file-here")
	fileTest, err := os.Create(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	baseTime := "2021-01-01T00:00:00Z"
	setMtime, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	setAtime, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	os.Chtimes(existingFile, setAtime, setMtime)
	fileHandler, _ := fileTest.Stat()
	mt := fileHandler.ModTime()
	at, _ := atime.Stat(existingFile)
	f := File{
		id:     "",
		method: "touch",
		params: map[string]interface{}{
			"name":  existingFile,
			"atime": baseTime,
			"mtime": baseTime,
		},
	}
	res, _ := f.touch(context.TODO(), true)
	notes := res.Notes
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
	note := fmt.Sprintf("file `%s` already has provided timestamps", existingFile)
	if notes[0].String() != note {
		t.Errorf("expected note `%s`, got `%s`", note, notes[0])
	}
	testHandler, _ := fileTest.Stat()
	tmt := testHandler.ModTime()
	tat, _ := atime.Stat(existingFile)
	if !mt.Equal(tmt) {
		t.Errorf("expected mtime to be %v, got %v", mt.UTC(), tmt.UTC())
	}
	if !at.Equal(tat) {
		t.Errorf("expected atime to be %v, got %v", at.UTC(), tat.UTC())
	}
}
