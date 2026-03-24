// SPDX-License-Identifier: BSD-3-Clause

// Copyright (C) 2017-2025 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securejoin

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cyphar/filepath-securejoin/internal/testutils"
)

// TODO: These tests won't work on plan9 because it doesn't have symlinks, and
//       also we use '/' here explicitly which probably won't work on Windows.

type input struct {
	root, unsafe string
	expected     string
}

func expandedTempDir(t *testing.T) string {
	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return dir
}

// Test basic handling of symlink expansion.
func TestSymlink(t *testing.T) {
	dir := expandedTempDir(t)

	testutils.Symlink(t, "somepath", filepath.Join(dir, "etc"))
	testutils.Symlink(t, "../../../../../../../../../../../../../etc", filepath.Join(dir, "etclink"))
	testutils.Symlink(t, "/../../../../../../../../../../../../../etc/passwd", filepath.Join(dir, "passwd"))

	rootOrVol := string(filepath.Separator)
	if vol := filepath.VolumeName(dir); vol != "" {
		rootOrVol = vol + rootOrVol
	}

	tc := []input{
		// Make sure that expansion with a root of '/' proceeds in the expected fashion.
		{rootOrVol, filepath.Join(dir, "passwd"), filepath.Join(rootOrVol, "etc", "passwd")},
		{rootOrVol, filepath.Join(dir, "etclink"), filepath.Join(rootOrVol, "etc")},

		{rootOrVol, filepath.Join(dir, "etc"), filepath.Join(dir, "somepath")},
		// Now test scoped expansion.
		{dir, "passwd", filepath.Join(dir, "somepath", "passwd")},
		{dir, "etclink", filepath.Join(dir, "somepath")},
		{dir, "etc", filepath.Join(dir, "somepath")},
		{dir, "etc/test", filepath.Join(dir, "somepath", "test")},
		{dir, "etc/test/..", filepath.Join(dir, "somepath")},
	}

	for _, test := range tc {
		got, err := SecureJoin(test.root, test.unsafe)
		if err != nil {
			t.Errorf("securejoin(%q, %q): unexpected error: %v", test.root, test.unsafe, err)
			continue
		}
		// This is only for OS X, where /etc is a symlink to /private/etc. In
		// principle, SecureJoin(/, pth) is the same as EvalSymlinks(pth) in
		// the case where the path exists.
		if test.root == "/" {
			if expected, err := filepath.EvalSymlinks(test.expected); err == nil {
				test.expected = expected
			}
		}
		if got != test.expected {
			t.Errorf("securejoin(%q, %q): expected %q, got %q", test.root, test.unsafe, test.expected, got)
			continue
		}
	}
}

// In a path without symlinks, SecureJoin is equivalent to Clean+Join.
func TestNoSymlink(t *testing.T) {
	dir := expandedTempDir(t)

	tc := []input{
		{dir, "somepath", filepath.Join(dir, "somepath")},
		{dir, "even/more/path", filepath.Join(dir, "even", "more", "path")},
		{dir, "/this/is/a/path", filepath.Join(dir, "this", "is", "a", "path")},
		{dir, "also/a/../path/././/with/some/./.././junk", filepath.Join(dir, "also", "path", "with", "junk")},
		{dir, "yetanother/../path/././/with/some/./.././junk../../../../../../../../../../../../etc/passwd", filepath.Join(dir, "etc", "passwd")},
		{dir, "/../../../../../../../../../../../../../../../../etc/passwd", filepath.Join(dir, "etc", "passwd")},
		{dir, "../../../../../../../../../../../../../../../../somedir", filepath.Join(dir, "somedir")},
		{dir, "../../../../../../../../../../../../../../../../", filepath.Join(dir)},
		{dir, "./../../.././././../../../../../../../../../../../../../../../../etc passwd", filepath.Join(dir, "etc passwd")},
	}

	if runtime.GOOS == "windows" {
		tc = append(tc, []input{
			{dir, "d:\\etc\\test", filepath.Join(dir, "etc", "test")},
		}...)
	}

	for _, test := range tc {
		got, err := SecureJoin(test.root, test.unsafe)
		if err != nil {
			t.Errorf("securejoin(%q, %q): unexpected error: %v", test.root, test.unsafe, err)
		}
		if got != test.expected {
			t.Errorf("securejoin(%q, %q): expected %q, got %q", test.root, test.unsafe, test.expected, got)
		}
	}
}

// Make sure that .. is **not** expanded lexically.
func TestNonLexical(t *testing.T) {
	dir := expandedTempDir(t)

	testutils.MkdirAll(t, filepath.Join(dir, "subdir"), 0o755)
	testutils.MkdirAll(t, filepath.Join(dir, "cousinparent", "cousin"), 0o755)
	testutils.Symlink(t, "../cousinparent/cousin", filepath.Join(dir, "subdir", "link"))
	testutils.Symlink(t, "/../cousinparent/cousin", filepath.Join(dir, "subdir", "link2"))
	testutils.Symlink(t, "/../../../../../../../../../../../../../../../../cousinparent/cousin", filepath.Join(dir, "subdir", "link3"))

	for _, test := range []input{
		{dir, "subdir", filepath.Join(dir, "subdir")},
		{dir, "subdir/link/test", filepath.Join(dir, "cousinparent", "cousin", "test")},
		{dir, "subdir/link2/test", filepath.Join(dir, "cousinparent", "cousin", "test")},
		{dir, "subdir/link3/test", filepath.Join(dir, "cousinparent", "cousin", "test")},
		{dir, "subdir/../test", filepath.Join(dir, "test")},
		// This is the divergence from a simple filepath.Clean implementation.
		{dir, "subdir/link/../test", filepath.Join(dir, "cousinparent", "test")},
		{dir, "subdir/link2/../test", filepath.Join(dir, "cousinparent", "test")},
		{dir, "subdir/link3/../test", filepath.Join(dir, "cousinparent", "test")},
	} {
		got, err := SecureJoin(test.root, test.unsafe)
		if err != nil {
			t.Errorf("securejoin(%q, %q): unexpected error: %v", test.root, test.unsafe, err)
			continue
		}
		if got != test.expected {
			t.Errorf("securejoin(%q, %q): expected %q, got %q", test.root, test.unsafe, test.expected, got)
			continue
		}
	}
}

// Make sure that symlink loops result in errors.
func TestSymlinkLoop(t *testing.T) {
	dir := expandedTempDir(t)

	testutils.MkdirAll(t, filepath.Join(dir, "subdir"), 0o755)
	testutils.Symlink(t, "../../../../../../../../../../../../../../../../path", filepath.Join(dir, "subdir", "link"))
	testutils.Symlink(t, "/subdir/link", filepath.Join(dir, "path"))
	testutils.Symlink(t, "/../../../../../../../../../../../../../../../../self", filepath.Join(dir, "self"))

	for _, test := range []struct {
		root, unsafe string
	}{
		{dir, "subdir/link"},
		{dir, "path"},
		{dir, "../../path"},
		{dir, "subdir/link/../.."},
		{dir, "../../../../../../../../../../../../../../../../subdir/link/../../../../../../../../../../../../../../../.."},
		{dir, "self"},
		{dir, "self/.."},
		{dir, "/../../../../../../../../../../../../../../../../self/.."},
		{dir, "/self/././.."},
	} {
		got, err := SecureJoin(test.root, test.unsafe)
		if !errors.Is(err, syscall.ELOOP) {
			t.Errorf("securejoin(%q, %q): expected ELOOP, got %q & %v", test.root, test.unsafe, got, err)
			continue
		}
	}
}

// Make sure that ENOTDIR is correctly handled.
func TestEnotdir(t *testing.T) {
	dir := expandedTempDir(t)

	testutils.MkdirAll(t, filepath.Join(dir, "subdir"), 0o755)
	testutils.WriteFile(t, filepath.Join(dir, "notdir"), []byte("I am not a directory!"), 0o755)
	testutils.Symlink(t, "/../../../notdir/somechild", filepath.Join(dir, "subdir", "link"))

	for _, test := range []struct {
		root, unsafe string
	}{
		{dir, "subdir/link"},
		{dir, "notdir"},
		{dir, "notdir/child"},
	} {
		_, err := SecureJoin(test.root, test.unsafe)
		if err != nil {
			t.Errorf("securejoin(%q, %q): unexpected error: %v", test.root, test.unsafe, err)
			continue
		}
	}
}

// Some silly tests to make sure that all error types are correctly handled.
func TestIsNotExist(t *testing.T) {
	for _, test := range []struct {
		err      error
		expected bool
	}{
		{&os.PathError{Op: "test1", Err: syscall.ENOENT}, true},
		{&os.LinkError{Op: "test1", Err: syscall.ENOENT}, true},
		{&os.SyscallError{Syscall: "test1", Err: syscall.ENOENT}, true},
		{&os.PathError{Op: "test2", Err: syscall.ENOTDIR}, true},
		{&os.LinkError{Op: "test2", Err: syscall.ENOTDIR}, true},
		{&os.SyscallError{Syscall: "test2", Err: syscall.ENOTDIR}, true},
		{&os.PathError{Op: "test3", Err: syscall.EACCES}, false},
		{&os.LinkError{Op: "test3", Err: syscall.EACCES}, false},
		{&os.SyscallError{Syscall: "test3", Err: syscall.EACCES}, false},
		{errors.New("not a proper error"), false},
	} {
		got := IsNotExist(test.err)
		if got != test.expected {
			t.Errorf("IsNotExist(%#v): expected %v, got %v", test.err, test.expected, got)
		}
	}
}

type mockVFS struct {
	lstat    func(path string) (os.FileInfo, error)
	readlink func(path string) (string, error)
}

func (m mockVFS) Lstat(path string) (os.FileInfo, error) { return m.lstat(path) }
func (m mockVFS) Readlink(path string) (string, error)   { return m.readlink(path) }

// Make sure that SecureJoinVFS actually does use the given VFS interface.
func TestSecureJoinVFS(t *testing.T) {
	dir := expandedTempDir(t)

	testutils.MkdirAll(t, filepath.Join(dir, "subdir"), 0o755)
	testutils.MkdirAll(t, filepath.Join(dir, "cousinparent", "cousin"), 0o755)
	testutils.Symlink(t, "../cousinparent/cousin", filepath.Join(dir, "subdir", "link"))
	testutils.Symlink(t, "/../cousinparent/cousin", filepath.Join(dir, "subdir", "link2"))
	testutils.Symlink(t, "/../../../../../../../../../../../../../../../../cousinparent/cousin", filepath.Join(dir, "subdir", "link3"))

	for _, test := range []input{
		{dir, "subdir", filepath.Join(dir, "subdir")},
		{dir, "subdir/link/test", filepath.Join(dir, "cousinparent", "cousin", "test")},
		{dir, "subdir/link2/test", filepath.Join(dir, "cousinparent", "cousin", "test")},
		{dir, "subdir/link3/test", filepath.Join(dir, "cousinparent", "cousin", "test")},
		{dir, "subdir/../test", filepath.Join(dir, "test")},
		// This is the divergence from a simple filepath.Clean implementation.
		{dir, "subdir/link/../test", filepath.Join(dir, "cousinparent", "test")},
		{dir, "subdir/link2/../test", filepath.Join(dir, "cousinparent", "test")},
		{dir, "subdir/link3/../test", filepath.Join(dir, "cousinparent", "test")},
	} {
		var nLstat, nReadlink int
		mock := mockVFS{
			lstat:    func(path string) (os.FileInfo, error) { nLstat++; return os.Lstat(path) },
			readlink: func(path string) (string, error) { nReadlink++; return os.Readlink(path) },
		}

		got, err := SecureJoinVFS(test.root, test.unsafe, mock)
		if err != nil {
			t.Errorf("securejoin(%q, %q): unexpected error: %v", test.root, test.unsafe, err)
			continue
		}
		if got != test.expected {
			t.Errorf("securejoin(%q, %q): expected %q, got %q", test.root, test.unsafe, test.expected, got)
			continue
		}
		if nLstat == 0 && nReadlink == 0 {
			t.Errorf("securejoin(%q, %q): expected to use either lstat or readlink, neither were used", test.root, test.unsafe)
		}
	}
}

// Make sure that SecureJoinVFS actually does use the given VFS interface, and
// that errors are correctly propagated.
func TestSecureJoinVFSErrors(t *testing.T) {
	var (
		lstatErr    = errors.New("lstat error")
		readlinkErr = errors.New("readlink err")
	)

	dir := expandedTempDir(t)

	// Make a link.
	testutils.Symlink(t, "../../../../../../../../../../../../../../../../path", filepath.Join(dir, "link"))

	// Define some fake mock functions.
	lstatFailFn := func(string) (os.FileInfo, error) { return nil, lstatErr }
	readlinkFailFn := func(string) (string, error) { return "", readlinkErr }

	// Make sure that the set of {lstat, readlink} failures do propagate.
	for idx, test := range []struct {
		vfs      VFS
		expected []error
	}{
		{
			expected: []error{nil},
			vfs: mockVFS{
				lstat:    os.Lstat,
				readlink: os.Readlink,
			},
		},
		{
			expected: []error{lstatErr},
			vfs: mockVFS{
				lstat:    lstatFailFn,
				readlink: os.Readlink,
			},
		},
		{
			expected: []error{readlinkErr},
			vfs: mockVFS{
				lstat:    os.Lstat,
				readlink: readlinkFailFn,
			},
		},
		{
			expected: []error{lstatErr, readlinkErr},
			vfs: mockVFS{
				lstat:    lstatFailFn,
				readlink: readlinkFailFn,
			},
		},
	} {
		_, err := SecureJoinVFS(dir, "link", test.vfs)

		success := false
		for _, exp := range test.expected {
			if errors.Is(err, exp) {
				success = true
			}
		}
		if !success {
			t.Errorf("SecureJoinVFS.mock%d: expected to get lstatError, got %v", idx, err)
		}
	}
}

func TestUncleanRoot(t *testing.T) {
	root := t.TempDir()

	for _, test := range []struct {
		testName, root string
		expectedErr    error
	}{
		{"trailing-dotdot", "foo/..", errUnsafeRoot},
		{"leading-dotdot", "../foo", errUnsafeRoot},
		{"middle-dotdot", "../foo", errUnsafeRoot},
		{"many-dotdot", "foo/../foo/../a", errUnsafeRoot},
		{"trailing-slash", root + "/foo/bar/", nil},
		{"trailing-slashes", root + "/foo/bar///", nil},
		{"many-slashes", root + "/foo///bar////baz", nil},
		{"plain-dot", root + "/foo/./bar", nil},
		{"many-dot", root + "/foo/./bar/./.", nil},
		{"unclean-safe", root + "/foo///./bar/.///.///", nil},
		{"unclean-unsafe", root + "/foo///./bar/..///.///", errUnsafeRoot},
	} {
		test := test // copy iterator
		t.Run(test.testName, func(t *testing.T) {
			_, err := SecureJoin(test.root, "foo/bar/baz")
			if test.expectedErr != nil {
				assert.ErrorIsf(t, err, test.expectedErr, "SecureJoin with unsafe root %q", test.root)
			} else {
				assert.NoErrorf(t, err, "SecureJoin with safe but unclean root %q", test.root)
			}
		})
	}
}

func TestHasDotDot(t *testing.T) {
	for _, test := range []struct {
		testName, path string
		expected       bool
	}{
		{"plain-dotdot", "..", true},
		{"trailing-dotdot", "foo/bar/baz/..", true},
		{"leading-dotdot", "../foo/bar/baz", true},
		{"middle-dotdot", "foo/bar/../baz", true},
		{"dotdot-in-name1", "foo/..bar/baz", false},
		{"dotdot-in-name2", "foo/bar../baz", false},
		{"dotdot-in-name3", "foo/b..r/baz", false},
		{"dotdot-in-name4", "..foo/bar/baz", false},
		{"dotdot-in-name5", "foo/bar/baz..", false},
		{"dot1", "./foo/bar/baz", false},
		{"dot2", "foo/bar/baz/.", false},
		{"dot3", "foo/././bar/baz", false},
		{"unclean", "foo//.//bar/baz////", false},
	} {
		test := test // copy iterator
		t.Run(test.testName, func(t *testing.T) {
			got := hasDotDot(test.path)
			assert.Equalf(t, test.expected, got, "unexpected result for hasDotDot(%q)", test.path)
		})
	}
}
