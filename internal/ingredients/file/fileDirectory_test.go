package file

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
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
			error: fmt.Errorf("directory %s does not exist and makedirs is disabled: %s", fileModeDNE, ErrPathNotFound),
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
			error: fmt.Errorf("directory %s does not exist and makedirs is disabled: %s", fileModeDNE, ErrPathNotFound),
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

// TestDirectoryApplyUserChown tests the apply path for chown to a named user.
// Covers the code path referenced in issue #31.
func TestDirectoryApplyUserChown(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "user-chown-dir")
	os.Mkdir(targetDir, 0o755)

	t.Run("ApplyUserChown", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name": targetDir,
				"user": currentUser.Username,
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes: []fmt.Stringer{
				cook.Snprintf("directory %s already exists", targetDir),
				cook.Snprintf("chown %s to %s", targetDir, currentUser.Username),
			},
		})
		stat, statErr := os.Stat(targetDir)
		if statErr != nil {
			t.Fatalf("failed to stat directory: %v", statErr)
		}
		sysStat := stat.Sys().(*syscall.Stat_t)
		expectedUID, _ := strconv.ParseUint(currentUser.Uid, 10, 32)
		if sysStat.Uid != uint32(expectedUID) {
			t.Errorf("expected uid %d, got %d", expectedUID, sysStat.Uid)
		}
	})

	t.Run("TestModeUserChown", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name": targetDir,
				"user": currentUser.Username,
			},
		}
		result, err := f.directory(context.Background(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes: []fmt.Stringer{
				cook.Snprintf("directory %s already exists", targetDir),
				cook.Snprintf("would chown %s to %s", targetDir, currentUser.Username),
			},
		})
	})

	t.Run("InvalidUser", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name": targetDir,
				"user": "nonexistent_user_xyzzy_12345",
			},
		}
		result, err := f.directory(context.Background(), false)
		if err == nil {
			t.Fatal("expected error for nonexistent user, got nil")
		}
		if result.Succeeded || !result.Failed {
			t.Errorf("expected failed result for nonexistent user")
		}
	})
}

// TestDirectoryApplyGroupChown tests the apply path for chown to a named group.
// Covers the code path referenced in issue #32.
func TestDirectoryApplyGroupChown(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}
	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		t.Fatalf("failed to get current group: %v", err)
	}
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "group-chown-dir")
	os.Mkdir(targetDir, 0o755)

	t.Run("ApplyGroupChown", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":  targetDir,
				"group": currentGroup.Name,
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes: []fmt.Stringer{
				cook.Snprintf("directory %s already exists", targetDir),
				cook.Snprintf("chown %s to %s", targetDir, currentGroup.Name),
			},
		})
		stat, statErr := os.Stat(targetDir)
		if statErr != nil {
			t.Fatalf("failed to stat directory: %v", statErr)
		}
		sysStat := stat.Sys().(*syscall.Stat_t)
		expectedGID, _ := strconv.ParseUint(currentGroup.Gid, 10, 32)
		if sysStat.Gid != uint32(expectedGID) {
			t.Errorf("expected gid %d, got %d", expectedGID, sysStat.Gid)
		}
	})

	t.Run("TestModeGroupChown", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":  targetDir,
				"group": currentGroup.Name,
			},
		}
		result, err := f.directory(context.Background(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes: []fmt.Stringer{
				cook.Snprintf("directory %s already exists", targetDir),
				cook.Snprintf("would chown %s to %s", targetDir, currentGroup.Name),
			},
		})
	})

	t.Run("InvalidGroup", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":  targetDir,
				"group": "nonexistent_group_xyzzy_12345",
			},
		}
		result, err := f.directory(context.Background(), false)
		if err == nil {
			t.Fatal("expected error for nonexistent group, got nil")
		}
		if result.Succeeded || !result.Failed {
			t.Errorf("expected failed result for nonexistent group")
		}
	})
}

// TestDirectoryApplyDirMode tests the apply path for chmod with dir_mode.
// Covers the code path referenced in issue #33.
func TestDirectoryApplyDirMode(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "dir-mode-dir")
	os.Mkdir(targetDir, 0o755)

	t.Run("ApplyDirMode700", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":     targetDir,
				"dir_mode": "700",
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes: []fmt.Stringer{
				cook.Snprintf("directory %s already exists", targetDir),
				cook.Snprintf("chmod %s to 700", targetDir),
			},
		})
		stat, statErr := os.Stat(targetDir)
		if statErr != nil {
			t.Fatalf("failed to stat directory: %v", statErr)
		}
		if stat.Mode().Perm() != 0o700 {
			t.Errorf("expected mode 0700, got %o", stat.Mode().Perm())
		}
	})

	t.Run("ApplyDirMode750", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":     targetDir,
				"dir_mode": "750",
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
		stat, statErr := os.Stat(targetDir)
		if statErr != nil {
			t.Fatalf("failed to stat directory: %v", statErr)
		}
		if stat.Mode().Perm() != 0o750 {
			t.Errorf("expected mode 0750, got %o", stat.Mode().Perm())
		}
	})

	t.Run("InvalidDirMode", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":     targetDir,
				"dir_mode": "not-a-mode",
			},
		}
		_, err := f.directory(context.Background(), false)
		if err == nil {
			t.Fatal("expected error for invalid dir_mode, got nil")
		}
	})
}

// TestDirectoryApplyFileMode tests the apply path for chmod with file_mode.
// Covers the code path referenced in issue #34.
func TestDirectoryApplyFileMode(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "file-mode-dir")
	os.Mkdir(targetDir, 0o755)

	t.Run("ApplyFileMode644", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":      targetDir,
				"file_mode": "644",
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
		stat, statErr := os.Stat(targetDir)
		if statErr != nil {
			t.Fatalf("failed to stat directory: %v", statErr)
		}
		if stat.Mode().Perm() != 0o644 {
			t.Errorf("expected mode 0644, got %o", stat.Mode().Perm())
		}
	})

	t.Run("ApplyFileMode600", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":      targetDir,
				"file_mode": "600",
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
		stat, statErr := os.Stat(targetDir)
		if statErr != nil {
			t.Fatalf("failed to stat directory: %v", statErr)
		}
		if stat.Mode().Perm() != 0o600 {
			t.Errorf("expected mode 0600, got %o", stat.Mode().Perm())
		}
	})

	t.Run("InvalidFileMode", func(t *testing.T) {
		f := File{
			params: map[string]interface{}{
				"name":      targetDir,
				"file_mode": "not-a-mode",
			},
		}
		_, err := f.directory(context.Background(), false)
		if err == nil {
			t.Fatal("expected error for invalid file_mode, got nil")
		}
	})
}

// TestDirectoryApplyRecurse tests the apply path for recursive chown operations.
// Covers the code path referenced in issue #35.
func TestDirectoryApplyRecurse(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}
	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		t.Fatalf("failed to get current group: %v", err)
	}

	t.Run("RecurseGroupChown", func(t *testing.T) {
		tempDir := t.TempDir()
		targetDir := filepath.Join(tempDir, "recurse-group-dir")
		os.Mkdir(targetDir, 0o755)
		subDir := filepath.Join(targetDir, "subdir")
		os.Mkdir(subDir, 0o755)
		subFile := filepath.Join(targetDir, "subfile")
		os.Create(subFile)

		f := File{
			params: map[string]interface{}{
				"name":    targetDir,
				"group":   currentGroup.Name,
				"recurse": true,
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
		stat, statErr := os.Stat(subDir)
		if statErr != nil {
			t.Fatalf("failed to stat subdir: %v", statErr)
		}
		sysStat := stat.Sys().(*syscall.Stat_t)
		expectedGID, _ := strconv.ParseUint(currentGroup.Gid, 10, 32)
		if sysStat.Gid != uint32(expectedGID) {
			t.Errorf("subdir: expected gid %d, got %d", expectedGID, sysStat.Gid)
		}
		stat, statErr = os.Stat(subFile)
		if statErr != nil {
			t.Fatalf("failed to stat subfile: %v", statErr)
		}
		sysStat = stat.Sys().(*syscall.Stat_t)
		if sysStat.Gid != uint32(expectedGID) {
			t.Errorf("subfile: expected gid %d, got %d", expectedGID, sysStat.Gid)
		}
	})

	t.Run("RecurseGroupChownTestMode", func(t *testing.T) {
		tempDir := t.TempDir()
		targetDir := filepath.Join(tempDir, "recurse-group-test-dir")
		os.Mkdir(targetDir, 0o755)
		subDir := filepath.Join(targetDir, "subdir")
		os.Mkdir(subDir, 0o755)

		f := File{
			params: map[string]interface{}{
				"name":    targetDir,
				"group":   currentGroup.Name,
				"recurse": true,
			},
		}
		result, err := f.directory(context.Background(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
		foundWouldChown := false
		for _, note := range result.Notes {
			if note.String() == fmt.Sprintf("would chown %s to %s", targetDir, currentGroup.Name) {
				foundWouldChown = true
				break
			}
		}
		if !foundWouldChown {
			t.Errorf("expected 'would chown' note in test mode, got notes: %v", result.Notes)
		}
	})

	t.Run("RecurseUserChown", func(t *testing.T) {
		tempDir := t.TempDir()
		targetDir := filepath.Join(tempDir, "recurse-user-dir")
		os.Mkdir(targetDir, 0o755)
		subDir := filepath.Join(targetDir, "subdir")
		os.Mkdir(subDir, 0o755)
		subFile := filepath.Join(targetDir, "subfile")
		os.Create(subFile)

		f := File{
			params: map[string]interface{}{
				"name":    targetDir,
				"user":    currentUser.Username,
				"recurse": true,
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
		stat, statErr := os.Stat(subDir)
		if statErr != nil {
			t.Fatalf("failed to stat subdir: %v", statErr)
		}
		sysStat := stat.Sys().(*syscall.Stat_t)
		expectedUID, _ := strconv.ParseUint(currentUser.Uid, 10, 32)
		if sysStat.Uid != uint32(expectedUID) {
			t.Errorf("subdir: expected uid %d, got %d", expectedUID, sysStat.Uid)
		}
	})

	t.Run("RecurseUserChownTestMode", func(t *testing.T) {
		tempDir := t.TempDir()
		targetDir := filepath.Join(tempDir, "recurse-user-test-dir")
		os.Mkdir(targetDir, 0o755)
		os.Mkdir(filepath.Join(targetDir, "subdir"), 0o755)

		f := File{
			params: map[string]interface{}{
				"name":    targetDir,
				"user":    currentUser.Username,
				"recurse": true,
			},
		}
		result, err := f.directory(context.Background(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Succeeded || result.Failed {
			t.Errorf("expected successful result")
		}
	})
}

// TestDirectoryApplyMakeDirs tests that makedirs creates new directories
// and that test mode does not create them.
func TestDirectoryApplyMakeDirs(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("CreateNewDirectory", func(t *testing.T) {
		newDir := filepath.Join(tempDir, "new", "nested", "dir")
		f := File{
			params: map[string]interface{}{
				"name":     newDir,
				"makedirs": true,
			},
		}
		result, err := f.directory(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes:     []fmt.Stringer{cook.Snprintf("created directory %s", newDir)},
		})
		stat, statErr := os.Stat(newDir)
		if statErr != nil {
			t.Fatalf("directory was not created: %v", statErr)
		}
		if !stat.IsDir() {
			t.Error("expected a directory, got a file")
		}
	})

	t.Run("TestModeNewDirectory", func(t *testing.T) {
		newDir := filepath.Join(tempDir, "test-new-dir")
		f := File{
			params: map[string]interface{}{
				"name":     newDir,
				"makedirs": true,
			},
		}
		result, err := f.directory(context.Background(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		compareResults(t, result, cook.Result{
			Succeeded: true,
			Failed:    false,
			Notes:     []fmt.Stringer{cook.Snprintf("would create directory %s", newDir)},
		})
		_, statErr := os.Stat(newDir)
		if statErr == nil {
			t.Error("directory should not be created in test mode")
		}
	})

	t.Run("MakedirsFalseReturnsError", func(t *testing.T) {
		newDir := filepath.Join(tempDir, "no-create-dir")
		f := File{
			params: map[string]interface{}{
				"name":     newDir,
				"makedirs": false,
			},
		}
		result, err := f.directory(context.Background(), false)
		if err == nil {
			t.Fatal("expected error when makedirs is false and dir does not exist")
		}
		if result.Succeeded || !result.Failed {
			t.Errorf("expected failed result")
		}
		_, statErr := os.Stat(newDir)
		if statErr == nil {
			t.Error("directory should not be created when makedirs is false")
		}
	})
}

// TestDirectoryApplyUserAndGroupCombined tests applying both user and group
// chown together in a single call.
func TestDirectoryApplyUserAndGroupCombined(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}
	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		t.Fatalf("failed to get current group: %v", err)
	}

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "combined-chown-dir")
	os.Mkdir(targetDir, 0o755)

	f := File{
		params: map[string]interface{}{
			"name":  targetDir,
			"user":  currentUser.Username,
			"group": currentGroup.Name,
		},
	}
	result, err := f.directory(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Errorf("expected successful result")
	}

	stat, statErr := os.Stat(targetDir)
	if statErr != nil {
		t.Fatalf("failed to stat directory: %v", statErr)
	}
	sysStat := stat.Sys().(*syscall.Stat_t)
	expectedUID, _ := strconv.ParseUint(currentUser.Uid, 10, 32)
	expectedGID, _ := strconv.ParseUint(currentGroup.Gid, 10, 32)
	if sysStat.Uid != uint32(expectedUID) {
		t.Errorf("expected uid %d, got %d", expectedUID, sysStat.Uid)
	}
	if sysStat.Gid != uint32(expectedGID) {
		t.Errorf("expected gid %d, got %d", expectedGID, sysStat.Gid)
	}
}

// TestDirectoryApplyAllParams tests a full apply with user, group, dir_mode,
// and file_mode all specified together.
func TestDirectoryApplyAllParams(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}
	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		t.Fatalf("failed to get current group: %v", err)
	}

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "all-params-dir")

	f := File{
		params: map[string]interface{}{
			"name":      targetDir,
			"user":      currentUser.Username,
			"group":     currentGroup.Name,
			"dir_mode":  "750",
			"file_mode": "640",
			"makedirs":  true,
		},
	}
	result, err := f.directory(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Succeeded || result.Failed {
		t.Errorf("expected successful result, got: %+v", result)
	}

	stat, statErr := os.Stat(targetDir)
	if statErr != nil {
		t.Fatalf("directory was not created: %v", statErr)
	}
	if !stat.IsDir() {
		t.Error("expected a directory")
	}
}
