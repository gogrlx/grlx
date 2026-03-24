// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pathrs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/procfs"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/testutils"
)

type openInRootFunc func(root, unsafePath string) (*os.File, error)

type openResult struct {
	handlePath string
	err        error
	fileType   uint32
}

// O_LARGEFILE is automatically added by the kernel when opening files on
// 64-bit machines. Unfortunately, it is architecture-dependent and
// unix.O_LARGEFILE is 0 (presumably to avoid users setting it). So we need to
// initialise it at init.
var O_LARGEFILE = 0x8000 //nolint:revive // unix.* name

func init() {
	switch runtime.GOARCH {
	case "arm", "arm64":
		O_LARGEFILE = 0x20000
	case "mips", "mips64", "mips64le", "mips64p32", "mips64p32le":
		O_LARGEFILE = 0x2000
	case "ppc", "ppc64", "ppc64le":
		O_LARGEFILE = 0x10000
	case "sparc", "sparc64":
		O_LARGEFILE = 0x40000
	default:
		// 0x8000 is the default flag in asm-generic.
	}
}

func tRunWrapper(t *testing.T) testutils.TRunFunc {
	return func(name string, doFn testutils.TDoFunc) {
		t.Run(name, func(t *testing.T) {
			doFn(t)
		})
	}
}

func checkReopen(t *testing.T, handle *os.File, flags int, expectedErr error) {
	newHandle, err := pathrs.Reopen(handle, flags)
	if newHandle != nil {
		defer newHandle.Close() //nolint:errcheck // test code
	}
	if expectedErr != nil {
		if assert.Error(t, err) {
			require.ErrorIs(t, err, expectedErr)
		} else {
			t.Errorf("unexpected handle %q", handle.Name())
		}
		return
	}
	require.NoError(t, err)

	// Get the original handle path.
	handlePath, err := procfs.ProcSelfFdReadlink(handle)
	require.NoError(t, err, "get real path of original handle")
	// Make sure the handle matches the readlink path.
	assert.Equal(t, handlePath, handle.Name(), "handle.Name() matching real original handle path")

	// Check that the new and old handle have the same path.
	newHandlePath, err := procfs.ProcSelfFdReadlink(newHandle)
	require.NoError(t, err, "get real path of reopened handle")
	assert.Equal(t, handlePath, newHandlePath, "old and reopen handle paths")
	assert.Equal(t, handle.Name(), newHandle.Name(), "old and reopen handle.Name()")

	// Check the fd flags.
	newHandleFdFlags, err := unix.FcntlInt(newHandle.Fd(), unix.F_GETFD, 0)
	require.NoError(t, err, "fcntl(F_GETFD)")
	assert.Equal(t, unix.FD_CLOEXEC, newHandleFdFlags&unix.FD_CLOEXEC, "FD_CLOEXEC flag must be set")

	// Check the file handle flags.
	newHandleStatusFlags, err := unix.FcntlInt(newHandle.Fd(), unix.F_GETFL, 0)
	require.NoError(t, err, "fcntl(F_GETFL)")
	flags &^= unix.O_CLOEXEC             // O_CLOEXEC is checked by F_GETFD
	newHandleStatusFlags &^= O_LARGEFILE // ignore the O_LARGEFILE flag
	assert.Equal(t, flags, newHandleStatusFlags, "re-opened handle status flags must match re-open flags (%+x)")
}

func checkOpenInRoot(t *testing.T, openInRootFn openInRootFunc, root, unsafePath string, expected openResult) {
	handle, err := openInRootFn(root, unsafePath)
	if handle != nil {
		defer handle.Close() //nolint:errcheck // test code
	}
	if expected.err != nil {
		if assert.Error(t, err) {
			require.ErrorIs(t, err, expected.err)
		} else {
			t.Errorf("unexpected handle %q", handle.Name())
		}
		return
	}
	require.NoError(t, err)

	// Check the handle path.
	gotPath, err := procfs.ProcSelfFdReadlink(handle)
	require.NoError(t, err, "get real path of returned handle")
	assert.Equal(t, expected.handlePath, gotPath, "real handle path")
	// Make sure the handle matches the readlink path.
	assert.Equal(t, gotPath, handle.Name(), "handle.Name() matching real handle path")

	// Check the handle type.
	unixStat, err := fd.Fstat(handle)
	require.NoError(t, err, "fstat handle")
	assert.Equal(t, expected.fileType, unixStat.Mode&unix.S_IFMT, "handle S_IFMT type")

	// Check that re-opening produces a handle with the same path.
	switch expected.fileType {
	case unix.S_IFDIR:
		checkReopen(t, handle, unix.O_RDONLY, nil)
		checkReopen(t, handle, unix.O_DIRECTORY, nil)
	case unix.S_IFREG:
		checkReopen(t, handle, unix.O_RDWR, nil)
		checkReopen(t, handle, unix.O_DIRECTORY, unix.ENOTDIR)
	// Only files and directories are safe to open this way. Use O_PATH for
	// everything else.
	default:
		checkReopen(t, handle, unix.O_PATH, nil)
		checkReopen(t, handle, unix.O_PATH|unix.O_DIRECTORY, unix.ENOTDIR)
	}
}

func testOpenInRoot(t *testing.T, openInRootFn openInRootFunc) {
	tree := []string{
		"dir a",
		"dir b/c/d/e/f",
		"file b/c/file",
		"symlink e /b/c/d/e",
		"symlink b-file b/c/file",
		// Dangling symlinks.
		"symlink a-fake1 a/fake",
		"symlink a-fake2 a/fake/foo/bar/..",
		"symlink a-fake3 a/fake/../../b",
		"dir c",
		"symlink c/a-fake1 a/fake",
		"symlink c/a-fake2 a/fake/foo/bar/..",
		"symlink c/a-fake3 a/fake/../../b",
		// Test non-lexical symlinks.
		"dir target",
		"dir link1",
		"symlink link1/target_abs /target",
		"symlink link1/target_rel ../target",
		"dir link2",
		"symlink link2/link1_abs /link1",
		"symlink link2/link1_rel ../link1",
		"dir link3",
		"symlink link3/target_abs /link2/link1_rel/target_rel",
		"symlink link3/target_rel ../link2/link1_rel/target_rel",
		"symlink link3/deep_dangling1 ../link2/link1_rel/target_rel/nonexist",
		"symlink link3/deep_dangling2 ../link2/link1_rel/target_rel/nonexist",
		// Deep dangling symlinks (with single components).
		"dir dangling",
		"symlink dangling/a b/c",
		"dir dangling/b",
		"symlink dangling/b/c ../c",
		"symlink dangling/c d/e",
		"dir dangling/d",
		"symlink dangling/d/e ../e",
		"symlink dangling/e f/../g",
		"dir dangling/f",
		"symlink dangling/g h/i/j/nonexistent",
		"dir dangling/h/i/j",
		// Deep dangling symlink using a non-dir component.
		"dir dangling-file",
		"symlink dangling-file/a b/c",
		"dir dangling-file/b",
		"symlink dangling-file/b/c ../c",
		"symlink dangling-file/c d/e",
		"dir dangling-file/d",
		"symlink dangling-file/d/e ../e",
		"symlink dangling-file/e f/../g",
		"dir dangling-file/f",
		"symlink dangling-file/g h/i/j/file/foo",
		"dir dangling-file/h/i/j",
		"file dangling-file/h/i/j/file",
		// Some "bad" inodes that a regular user can create.
		"fifo b/fifo",
		"sock b/sock",
		// Symlink loops.
		"dir loop",
		"symlink loop/basic-loop1 basic-loop1",
		"symlink loop/basic-loop2 /loop/basic-loop2",
		"symlink loop/basic-loop3 ../loop/basic-loop3",
		"dir loop/a",
		"symlink loop/a/link ../b/link",
		"dir loop/b",
		"symlink loop/b/link /loop/c/link",
		"dir loop/c",
		"symlink loop/c/link /loop/d/link",
		"symlink loop/d e",
		"dir loop/e",
		"symlink loop/e/link ../a/link",
		"symlink loop/link a/link",
	}

	root := testutils.CreateTree(t, tree...)

	for name, test := range map[string]struct {
		unsafePath string
		expected   openResult
	}{
		// Complete lookups.
		"complete-dir1":      {"a", openResult{handlePath: "/a", fileType: unix.S_IFDIR}},
		"complete-dir2":      {"b/c/d/e/f", openResult{handlePath: "/b/c/d/e/f", fileType: unix.S_IFDIR}},
		"complete-dir3":      {"b///././c////.//d/./././///e////.//./f//././././", openResult{handlePath: "/b/c/d/e/f", fileType: unix.S_IFDIR}},
		"complete-file":      {"b/c/file", openResult{handlePath: "/b/c/file", fileType: unix.S_IFREG}},
		"complete-file-link": {"b-file", openResult{handlePath: "/b/c/file", fileType: unix.S_IFREG}},
		"complete-fifo":      {"b/fifo", openResult{handlePath: "/b/fifo", fileType: unix.S_IFIFO}},
		"complete-sock":      {"b/sock", openResult{handlePath: "/b/sock", fileType: unix.S_IFSOCK}},
		// Partial lookups.
		"partial-dir-basic":  {"a/b/c/d/e/f/g/h", openResult{err: unix.ENOENT}},
		"partial-dir-dotdot": {"a/foo/../bar/baz", openResult{err: unix.ENOENT}},
		// Complete lookups of non-lexical symlinks.
		"nonlexical-basic-complete1":                {"target", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-basic-complete2":                {"target/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-basic-complete3":                {"target//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-basic-partial":                  {"target/foo", openResult{err: unix.ENOENT}},
		"nonlexical-basic-partial-dotdot":           {"target/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level1-abs-complete1":           {"link1/target_abs", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level1-abs-complete2":           {"link1/target_abs/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level1-abs-complete3":           {"link1/target_abs//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level1-abs-partial":             {"link1/target_abs/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level1-abs-partial-dotdot":      {"link1/target_abs/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level1-rel-complete1":           {"link1/target_rel", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level1-rel-complete2":           {"link1/target_rel/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level1-rel-complete3":           {"link1/target_rel//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level1-rel-partial":             {"link1/target_rel/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level1-rel-partial-dotdot":      {"link1/target_rel/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level2-abs-abs-complete1":       {"link2/link1_abs/target_abs", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-abs-complete2":       {"link2/link1_abs/target_abs/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-abs-complete3":       {"link2/link1_abs/target_abs//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-abs-partial":         {"link2/link1_abs/target_abs/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level2-abs-abs-partial-dotdot":  {"link2/link1_abs/target_abs/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level2-abs-rel-complete1":       {"link2/link1_abs/target_rel", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-rel-complete2":       {"link2/link1_abs/target_rel/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-rel-complete3":       {"link2/link1_abs/target_rel//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-rel-partial":         {"link2/link1_abs/target_rel/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level2-abs-rel-partial-dotdot":  {"link2/link1_abs/target_rel/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level2-abs-open-complete1":      {"link2/link1_abs/../target", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-open-complete2":      {"link2/link1_abs/../target/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-open-complete3":      {"link2/link1_abs/../target//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-open-partial":        {"link2/link1_abs/../target/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level2-abs-open-partial-dotdot": {"link2/link1_abs/../target/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level2-rel-abs-complete1":       {"link2/link1_rel/target_abs", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-abs-complete2":       {"link2/link1_rel/target_abs/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-abs-complete3":       {"link2/link1_rel/target_abs//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-abs-partial":         {"link2/link1_rel/target_abs/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level2-rel-abs-partial-dotdot":  {"link2/link1_rel/target_abs/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level2-rel-rel-complete1":       {"link2/link1_rel/target_rel", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-rel-complete2":       {"link2/link1_rel/target_rel/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-rel-complete3":       {"link2/link1_rel/target_rel//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-rel-partial":         {"link2/link1_rel/target_rel/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level2-rel-rel-partial-dotdot":  {"link2/link1_rel/target_rel/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level2-rel-open-complete1":      {"link2/link1_rel/../target", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-open-complete2":      {"link2/link1_rel/../target/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-open-complete3":      {"link2/link1_rel/../target//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-open-partial":        {"link2/link1_rel/../target/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level2-rel-open-partial-dotdot": {"link2/link1_rel/../target/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level3-abs-complete1":           {"link3/target_abs", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level3-abs-complete2":           {"link3/target_abs/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level3-abs-complete3":           {"link3/target_abs//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level3-abs-partial":             {"link3/target_abs/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level3-abs-partial-dotdot":      {"link3/target_abs/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		"nonlexical-level3-rel-complete1":           {"link3/target_rel", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level3-rel-complete2":           {"link3/target_rel/", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level3-rel-complete3":           {"link3/target_rel//", openResult{handlePath: "/target", fileType: unix.S_IFDIR}},
		"nonlexical-level3-rel-partial":             {"link3/target_rel/foo", openResult{err: unix.ENOENT}},
		"nonlexical-level3-rel-partial-dotdot":      {"link3/target_rel/../target/foo/bar/../baz", openResult{err: unix.ENOENT}},
		// Partial lookups due to hitting a non-directory.
		"partial-nondir-slash1":          {"b/c/file/", openResult{err: unix.ENOTDIR}},
		"partial-nondir-slash2":          {"b/c/file//", openResult{err: unix.ENOTDIR}},
		"partial-nondir-dot":             {"b/c/file/.", openResult{err: unix.ENOTDIR}},
		"partial-nondir-dotdot1":         {"b/c/file/..", openResult{err: unix.ENOTDIR}},
		"partial-nondir-dotdot2":         {"b/c/file/../foo/bar", openResult{err: unix.ENOTDIR}},
		"partial-nondir-symlink-slash1":  {"b-file/", openResult{err: unix.ENOTDIR}},
		"partial-nondir-symlink-slash2":  {"b-file//", openResult{err: unix.ENOTDIR}},
		"partial-nondir-symlink-dot":     {"b-file/.", openResult{err: unix.ENOTDIR}},
		"partial-nondir-symlink-dotdot1": {"b-file/..", openResult{err: unix.ENOTDIR}},
		"partial-nondir-symlink-dotdot2": {"b-file/../foo/bar", openResult{err: unix.ENOTDIR}},
		"partial-fifo-slash1":            {"b/fifo/", openResult{err: unix.ENOTDIR}},
		"partial-fifo-slash2":            {"b/fifo//", openResult{err: unix.ENOTDIR}},
		"partial-fifo-dot":               {"b/fifo/.", openResult{err: unix.ENOTDIR}},
		"partial-fifo-dotdot1":           {"b/fifo/..", openResult{err: unix.ENOTDIR}},
		"partial-fifo-dotdot2":           {"b/fifo/../foo/bar", openResult{err: unix.ENOTDIR}},
		"partial-sock-slash1":            {"b/sock/", openResult{err: unix.ENOTDIR}},
		"partial-sock-slash2":            {"b/sock//", openResult{err: unix.ENOTDIR}},
		"partial-sock-dot":               {"b/sock/.", openResult{err: unix.ENOTDIR}},
		"partial-sock-dotdot1":           {"b/sock/..", openResult{err: unix.ENOTDIR}},
		"partial-sock-dotdot2":           {"b/sock/../foo/bar", openResult{err: unix.ENOTDIR}},
		// Dangling symlinks are treated as though they are non-existent.
		"dangling1-inroot-trailing":       {"a-fake1", openResult{err: unix.ENOENT}},
		"dangling1-inroot-partial":        {"a-fake1/foo", openResult{err: unix.ENOENT}},
		"dangling1-inroot-partial-dotdot": {"a-fake1/../bar/baz", openResult{err: unix.ENOENT}},
		"dangling1-sub-trailing":          {"c/a-fake1", openResult{err: unix.ENOENT}},
		"dangling1-sub-partial":           {"c/a-fake1/foo", openResult{err: unix.ENOENT}},
		"dangling1-sub-partial-dotdot":    {"c/a-fake1/../bar/baz", openResult{err: unix.ENOENT}},
		"dangling2-inroot-trailing":       {"a-fake2", openResult{err: unix.ENOENT}},
		"dangling2-inroot-partial":        {"a-fake2/foo", openResult{err: unix.ENOENT}},
		"dangling2-inroot-partial-dotdot": {"a-fake2/../bar/baz", openResult{err: unix.ENOENT}},
		"dangling2-sub-trailing":          {"c/a-fake2", openResult{err: unix.ENOENT}},
		"dangling2-sub-partial":           {"c/a-fake2/foo", openResult{err: unix.ENOENT}},
		"dangling2-sub-partial-dotdot":    {"c/a-fake2/../bar/baz", openResult{err: unix.ENOENT}},
		"dangling3-inroot-trailing":       {"a-fake3", openResult{err: unix.ENOENT}},
		"dangling3-inroot-partial":        {"a-fake3/foo", openResult{err: unix.ENOENT}},
		"dangling3-inroot-partial-dotdot": {"a-fake3/../bar/baz", openResult{err: unix.ENOENT}},
		"dangling3-sub-trailing":          {"c/a-fake3", openResult{err: unix.ENOENT}},
		"dangling3-sub-partial":           {"c/a-fake3/foo", openResult{err: unix.ENOENT}},
		"dangling3-sub-partial-dotdot":    {"c/a-fake3/../bar/baz", openResult{err: unix.ENOENT}},
		// Tricky dangling symlinks.
		"dangling-tricky1-trailing":       {"link3/deep_dangling1", openResult{err: unix.ENOENT}},
		"dangling-tricky1-partial":        {"link3/deep_dangling1/foo", openResult{err: unix.ENOENT}},
		"dangling-tricky1-partial-dotdot": {"link3/deep_dangling1/..", openResult{err: unix.ENOENT}},
		"dangling-tricky2-trailing":       {"link3/deep_dangling2", openResult{err: unix.ENOENT}},
		"dangling-tricky2-partial":        {"link3/deep_dangling2/foo", openResult{err: unix.ENOENT}},
		"dangling-tricky2-partial-dotdot": {"link3/deep_dangling2/..", openResult{err: unix.ENOENT}},
		// Really deep dangling links.
		"deep-dangling1":           {"dangling/a", openResult{err: unix.ENOENT}},
		"deep-dangling2":           {"dangling/b/c", openResult{err: unix.ENOENT}},
		"deep-dangling3":           {"dangling/c", openResult{err: unix.ENOENT}},
		"deep-dangling4":           {"dangling/d/e", openResult{err: unix.ENOENT}},
		"deep-dangling5":           {"dangling/e", openResult{err: unix.ENOENT}},
		"deep-dangling6":           {"dangling/g", openResult{err: unix.ENOENT}},
		"deep-dangling-fileasdir1": {"dangling-file/a", openResult{err: unix.ENOTDIR}},
		"deep-dangling-fileasdir2": {"dangling-file/b/c", openResult{err: unix.ENOTDIR}},
		"deep-dangling-fileasdir3": {"dangling-file/c", openResult{err: unix.ENOTDIR}},
		"deep-dangling-fileasdir4": {"dangling-file/d/e", openResult{err: unix.ENOTDIR}},
		"deep-dangling-fileasdir5": {"dangling-file/e", openResult{err: unix.ENOTDIR}},
		"deep-dangling-fileasdir6": {"dangling-file/g", openResult{err: unix.ENOTDIR}},
		// Symlink loops.
		"loop":        {"loop/link", openResult{err: unix.ELOOP}},
		"loop-basic1": {"loop/basic-loop1", openResult{err: unix.ELOOP}},
		"loop-basic2": {"loop/basic-loop2", openResult{err: unix.ELOOP}},
		"loop-basic3": {"loop/basic-loop3", openResult{err: unix.ELOOP}},
	} {
		test := test // copy iterator
		// Update the handlePath to be inside our root.
		if test.expected.handlePath != "" {
			test.expected.handlePath = filepath.Join(root, test.expected.handlePath)
		}
		t.Run(name, func(t *testing.T) {
			checkOpenInRoot(t, openInRootFn, root, test.unsafePath, test.expected)
		})
	}
}

func TestOpenInRoot(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		testOpenInRoot(t, pathrs.OpenInRoot)
	})
}

func TestOpenInRootHandle(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		testOpenInRoot(t, func(root, unsafePath string) (*os.File, error) {
			rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
			if err != nil {
				return nil, err
			}
			defer rootDir.Close() //nolint:errcheck // test code

			return pathrs.OpenatInRoot(rootDir, unsafePath)
		})
	})
}

func TestOpenInRoot_BadRoot(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	t.Run("OpenInRoot", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "does/not/exist")

		handle, err := pathrs.OpenInRoot(root, ".")
		require.ErrorIs(t, err, os.ErrNotExist, "OpenInRoot with bad root")
		assert.Nil(t, handle, "OpenInRoot with bad root should not return handle")
	})
	// TODO: Should we add checks for nil *os.File?
}

func TestOpenInRoot_BadInode(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	testutils.RequireRoot(t) // mknod

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		tree := []string{
			// Make sure we don't open "bad" inodes.
			"dir foo",
			"char foo/whiteout 0 0",
			"block foo/whiteout-blk 0 0",
		}

		root := testutils.CreateTree(t, tree...)

		rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		defer rootDir.Close() //nolint:errcheck // test code

		for name, test := range map[string]struct {
			unsafePath string
			expected   openResult
		}{
			// Complete lookups.
			"char-trailing": {"foo/whiteout", openResult{handlePath: "/foo/whiteout", fileType: unix.S_IFCHR}},
			"blk-trailing":  {"foo/whiteout-blk", openResult{handlePath: "/foo/whiteout-blk", fileType: unix.S_IFBLK}},
			// Partial lookups due to hitting a non-directory.
			"char-dot":     {"foo/whiteout/.", openResult{err: unix.ENOTDIR}},
			"char-dotdot1": {"foo/whiteout/..", openResult{err: unix.ENOTDIR}},
			"char-dotdot2": {"foo/whiteout/../foo/bar", openResult{err: unix.ENOTDIR}},
			"blk-dot":      {"foo/whiteout-blk/.", openResult{err: unix.ENOTDIR}},
			"blk-dotdot1":  {"foo/whiteout-blk/..", openResult{err: unix.ENOTDIR}},
			"blk-dotdot2":  {"foo/whiteout-blk/../foo/bar", openResult{err: unix.ENOTDIR}},
		} {
			test := test // copy iterator
			// Update the handlePath to be inside our root.
			if test.expected.handlePath != "" {
				test.expected.handlePath = filepath.Join(root, test.expected.handlePath)
			}
			t.Run(name, func(t *testing.T) {
				checkOpenInRoot(t, pathrs.OpenInRoot, root, test.unsafePath, test.expected)
			})
		}
	})
}
