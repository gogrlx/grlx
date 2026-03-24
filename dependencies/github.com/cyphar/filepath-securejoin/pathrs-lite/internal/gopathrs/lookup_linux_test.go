// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package gopathrs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/gocompat"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/procfs"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/testutils"
)

type partialLookupFunc func(root fd.Fd, unsafePath string) (*os.File, string, error)

type lookupResult struct {
	handlePath, remainingPath string
	err                       error
	fileType                  uint32
}

func checkPartialLookup(t *testing.T, partialLookupFn partialLookupFunc, rootDir fd.Fd, unsafePath string, expected lookupResult) {
	handle, remainingPath, err := partialLookupFn(rootDir, unsafePath)
	if handle != nil {
		defer handle.Close() //nolint:errcheck // test code
	}
	if expected.err != nil {
		if assert.Error(t, err) {
			assert.ErrorIs(t, err, expected.err)
		}
		if expected.handlePath == "" {
			require.Nil(t, handle, "expected to not get a handle")
			return
		}
	} else {
		if expected.remainingPath != "" {
			t.Errorf("we expect a remaining path, but no error? %q", expected.remainingPath)
		}
		require.NoError(t, err)
	}
	assert.NotNil(t, handle, "expected to get a handle")

	// Check the remainingPath.
	assert.Equal(t, expected.remainingPath, remainingPath, "remaining path")

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
}

func testPartialLookup(t *testing.T, partialLookupFn partialLookupFunc) {
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

	rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	require.NoError(t, err)
	defer rootDir.Close() //nolint:errcheck // test code

	for name, test := range map[string]struct {
		unsafePath string
		expected   lookupResult
	}{
		// Complete lookups.
		"complete-dir1": {"a", lookupResult{handlePath: "/a", remainingPath: "", fileType: unix.S_IFDIR}},
		"complete-dir2": {"b/c/d/e/f", lookupResult{handlePath: "/b/c/d/e/f", remainingPath: "", fileType: unix.S_IFDIR}},
		"complete-dir3": {"b///././c////.//d/./././///e////.//./f//././././", lookupResult{handlePath: "/b/c/d/e/f", remainingPath: "", fileType: unix.S_IFDIR}},
		"complete-fifo": {"b/fifo", lookupResult{handlePath: "/b/fifo", remainingPath: "", fileType: unix.S_IFIFO}},
		"complete-sock": {"b/sock", lookupResult{handlePath: "/b/sock", remainingPath: "", fileType: unix.S_IFSOCK}},
		// Partial lookups.
		"partial-dir-basic":  {"a/b/c/d/e/f/g/h", lookupResult{handlePath: "/a", remainingPath: "b/c/d/e/f/g/h", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"partial-dir-dotdot": {"a/foo/../bar/baz", lookupResult{handlePath: "/a", remainingPath: "foo/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		// Complete lookups of non-lexical symlinks.
		"nonlexical-basic-complete1":                {"target", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-basic-complete2":                {"target/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-basic-complete3":                {"target//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-basic-partial":                  {"target/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-basic-partial-dotdot":           {"target/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level1-abs-complete1":           {"link1/target_abs", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level1-abs-complete2":           {"link1/target_abs/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level1-abs-complete3":           {"link1/target_abs//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level1-abs-partial":             {"link1/target_abs/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level1-abs-partial-dotdot":      {"link1/target_abs/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level1-rel-complete1":           {"link1/target_rel", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level1-rel-complete2":           {"link1/target_rel/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level1-rel-complete3":           {"link1/target_rel//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level1-rel-partial":             {"link1/target_rel/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level1-rel-partial-dotdot":      {"link1/target_rel/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-abs-abs-complete1":       {"link2/link1_abs/target_abs", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-abs-complete2":       {"link2/link1_abs/target_abs/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-abs-complete3":       {"link2/link1_abs/target_abs//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-abs-partial":         {"link2/link1_abs/target_abs/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-abs-abs-partial-dotdot":  {"link2/link1_abs/target_abs/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-abs-rel-complete1":       {"link2/link1_abs/target_rel", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-rel-complete2":       {"link2/link1_abs/target_rel/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-rel-complete3":       {"link2/link1_abs/target_rel//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-rel-partial":         {"link2/link1_abs/target_rel/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-abs-rel-partial-dotdot":  {"link2/link1_abs/target_rel/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-abs-open-complete1":      {"link2/link1_abs/../target", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-open-complete2":      {"link2/link1_abs/../target/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-open-complete3":      {"link2/link1_abs/../target//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-abs-open-partial":        {"link2/link1_abs/../target/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-abs-open-partial-dotdot": {"link2/link1_abs/../target/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-rel-abs-complete1":       {"link2/link1_rel/target_abs", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-abs-complete2":       {"link2/link1_rel/target_abs/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-abs-complete3":       {"link2/link1_rel/target_abs//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-abs-partial":         {"link2/link1_rel/target_abs/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-rel-abs-partial-dotdot":  {"link2/link1_rel/target_abs/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-rel-rel-complete1":       {"link2/link1_rel/target_rel", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-rel-complete2":       {"link2/link1_rel/target_rel/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-rel-complete3":       {"link2/link1_rel/target_rel//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-rel-partial":         {"link2/link1_rel/target_rel/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-rel-rel-partial-dotdot":  {"link2/link1_rel/target_rel/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-rel-open-complete1":      {"link2/link1_rel/../target", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-open-complete2":      {"link2/link1_rel/../target/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-open-complete3":      {"link2/link1_rel/../target//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level2-rel-open-partial":        {"link2/link1_rel/../target/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level2-rel-open-partial-dotdot": {"link2/link1_rel/../target/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level3-abs-complete1":           {"link3/target_abs", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level3-abs-complete2":           {"link3/target_abs/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level3-abs-complete3":           {"link3/target_abs//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level3-abs-partial":             {"link3/target_abs/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level3-abs-partial-dotdot":      {"link3/target_abs/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level3-rel-complete1":           {"link3/target_rel", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level3-rel-complete2":           {"link3/target_rel/", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level3-rel-complete3":           {"link3/target_rel//", lookupResult{handlePath: "/target", remainingPath: "", fileType: unix.S_IFDIR}},
		"nonlexical-level3-rel-partial":             {"link3/target_rel/foo", lookupResult{handlePath: "/target", remainingPath: "foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"nonlexical-level3-rel-partial-dotdot":      {"link3/target_rel/../target/foo/bar/../baz", lookupResult{handlePath: "/target", remainingPath: "foo/bar/../baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		// Partial lookups due to hitting a non-directory.
		"partial-nondir-slash1":          {"b/c/file/", lookupResult{handlePath: "/b/c/file", remainingPath: "", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-slash2":          {"b/c/file//", lookupResult{handlePath: "/b/c/file", remainingPath: "/", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-dot":             {"b/c/file/.", lookupResult{handlePath: "/b/c/file", remainingPath: ".", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-dotdot1":         {"b/c/file/..", lookupResult{handlePath: "/b/c/file", remainingPath: "..", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-dotdot2":         {"b/c/file/../foo/bar", lookupResult{handlePath: "/b/c/file", remainingPath: "../foo/bar", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-symlink-slash1":  {"b-file/", lookupResult{handlePath: "/b/c/file", remainingPath: "", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-symlink-slash2":  {"b-file//", lookupResult{handlePath: "/b/c/file", remainingPath: "/", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-symlink-dot":     {"b-file/.", lookupResult{handlePath: "/b/c/file", remainingPath: ".", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-symlink-dotdot1": {"b-file/..", lookupResult{handlePath: "/b/c/file", remainingPath: "..", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-nondir-symlink-dotdot2": {"b-file/../foo/bar", lookupResult{handlePath: "/b/c/file", remainingPath: "../foo/bar", fileType: unix.S_IFREG, err: unix.ENOTDIR}},
		"partial-fifo-slash1":            {"b/fifo/", lookupResult{handlePath: "/b/fifo", remainingPath: "", fileType: unix.S_IFIFO, err: unix.ENOTDIR}},
		"partial-fifo-slash2":            {"b/fifo//", lookupResult{handlePath: "/b/fifo", remainingPath: "/", fileType: unix.S_IFIFO, err: unix.ENOTDIR}},
		"partial-fifo-dot":               {"b/fifo/.", lookupResult{handlePath: "/b/fifo", remainingPath: ".", fileType: unix.S_IFIFO, err: unix.ENOTDIR}},
		"partial-fifo-dotdot1":           {"b/fifo/..", lookupResult{handlePath: "/b/fifo", remainingPath: "..", fileType: unix.S_IFIFO, err: unix.ENOTDIR}},
		"partial-fifo-dotdot2":           {"b/fifo/../foo/bar", lookupResult{handlePath: "/b/fifo", remainingPath: "../foo/bar", fileType: unix.S_IFIFO, err: unix.ENOTDIR}},
		"partial-sock-slash1":            {"b/sock/", lookupResult{handlePath: "/b/sock", remainingPath: "", fileType: unix.S_IFSOCK, err: unix.ENOTDIR}},
		"partial-sock-slash2":            {"b/sock//", lookupResult{handlePath: "/b/sock", remainingPath: "/", fileType: unix.S_IFSOCK, err: unix.ENOTDIR}},
		"partial-sock-dot":               {"b/sock/.", lookupResult{handlePath: "/b/sock", remainingPath: ".", fileType: unix.S_IFSOCK, err: unix.ENOTDIR}},
		"partial-sock-dotdot1":           {"b/sock/..", lookupResult{handlePath: "/b/sock", remainingPath: "..", fileType: unix.S_IFSOCK, err: unix.ENOTDIR}},
		"partial-sock-dotdot2":           {"b/sock/../foo/bar", lookupResult{handlePath: "/b/sock", remainingPath: "../foo/bar", fileType: unix.S_IFSOCK, err: unix.ENOTDIR}},
		// Dangling symlinks are treated as though they are non-existent.
		"dangling1-inroot-trailing":       {"a-fake1", lookupResult{handlePath: "/", remainingPath: "a-fake1", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling1-inroot-partial":        {"a-fake1/foo", lookupResult{handlePath: "/", remainingPath: "a-fake1/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling1-inroot-partial-dotdot": {"a-fake1/../bar/baz", lookupResult{handlePath: "/", remainingPath: "a-fake1/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling1-sub-trailing":          {"c/a-fake1", lookupResult{handlePath: "/c", remainingPath: "a-fake1", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling1-sub-partial":           {"c/a-fake1/foo", lookupResult{handlePath: "/c", remainingPath: "a-fake1/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling1-sub-partial-dotdot":    {"c/a-fake1/../bar/baz", lookupResult{handlePath: "/c", remainingPath: "a-fake1/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling2-inroot-trailing":       {"a-fake2", lookupResult{handlePath: "/", remainingPath: "a-fake2", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling2-inroot-partial":        {"a-fake2/foo", lookupResult{handlePath: "/", remainingPath: "a-fake2/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling2-inroot-partial-dotdot": {"a-fake2/../bar/baz", lookupResult{handlePath: "/", remainingPath: "a-fake2/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling2-sub-trailing":          {"c/a-fake2", lookupResult{handlePath: "/c", remainingPath: "a-fake2", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling2-sub-partial":           {"c/a-fake2/foo", lookupResult{handlePath: "/c", remainingPath: "a-fake2/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling2-sub-partial-dotdot":    {"c/a-fake2/../bar/baz", lookupResult{handlePath: "/c", remainingPath: "a-fake2/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling3-inroot-trailing":       {"a-fake3", lookupResult{handlePath: "/", remainingPath: "a-fake3", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling3-inroot-partial":        {"a-fake3/foo", lookupResult{handlePath: "/", remainingPath: "a-fake3/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling3-inroot-partial-dotdot": {"a-fake3/../bar/baz", lookupResult{handlePath: "/", remainingPath: "a-fake3/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling3-sub-trailing":          {"c/a-fake3", lookupResult{handlePath: "/c", remainingPath: "a-fake3", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling3-sub-partial":           {"c/a-fake3/foo", lookupResult{handlePath: "/c", remainingPath: "a-fake3/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling3-sub-partial-dotdot":    {"c/a-fake3/../bar/baz", lookupResult{handlePath: "/c", remainingPath: "a-fake3/../bar/baz", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		// Tricky dangling symlinks.
		"dangling-tricky1-trailing":       {"link3/deep_dangling1", lookupResult{handlePath: "/link3", remainingPath: "deep_dangling1", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling-tricky1-partial":        {"link3/deep_dangling1/foo", lookupResult{handlePath: "/link3", remainingPath: "deep_dangling1/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling-tricky1-partial-dotdot": {"link3/deep_dangling1/..", lookupResult{handlePath: "/link3", remainingPath: "deep_dangling1/..", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling-tricky2-trailing":       {"link3/deep_dangling2", lookupResult{handlePath: "/link3", remainingPath: "deep_dangling2", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling-tricky2-partial":        {"link3/deep_dangling2/foo", lookupResult{handlePath: "/link3", remainingPath: "deep_dangling2/foo", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"dangling-tricky2-partial-dotdot": {"link3/deep_dangling2/..", lookupResult{handlePath: "/link3", remainingPath: "deep_dangling2/..", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		// Really deep dangling links.
		"deep-dangling1":           {"dangling/a", lookupResult{handlePath: "/dangling", remainingPath: "a", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"deep-dangling2":           {"dangling/b/c", lookupResult{handlePath: "/dangling/b", remainingPath: "c", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"deep-dangling3":           {"dangling/c", lookupResult{handlePath: "/dangling", remainingPath: "c", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"deep-dangling4":           {"dangling/d/e", lookupResult{handlePath: "/dangling/d", remainingPath: "e", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"deep-dangling5":           {"dangling/e", lookupResult{handlePath: "/dangling", remainingPath: "e", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"deep-dangling6":           {"dangling/g", lookupResult{handlePath: "/dangling", remainingPath: "g", fileType: unix.S_IFDIR, err: unix.ENOENT}},
		"deep-dangling-fileasdir1": {"dangling-file/a", lookupResult{handlePath: "/dangling-file", remainingPath: "a", fileType: unix.S_IFDIR, err: unix.ENOTDIR}},
		"deep-dangling-fileasdir2": {"dangling-file/b/c", lookupResult{handlePath: "/dangling-file/b", remainingPath: "c", fileType: unix.S_IFDIR, err: unix.ENOTDIR}},
		"deep-dangling-fileasdir3": {"dangling-file/c", lookupResult{handlePath: "/dangling-file", remainingPath: "c", fileType: unix.S_IFDIR, err: unix.ENOTDIR}},
		"deep-dangling-fileasdir4": {"dangling-file/d/e", lookupResult{handlePath: "/dangling-file/d", remainingPath: "e", fileType: unix.S_IFDIR, err: unix.ENOTDIR}},
		"deep-dangling-fileasdir5": {"dangling-file/e", lookupResult{handlePath: "/dangling-file", remainingPath: "e", fileType: unix.S_IFDIR, err: unix.ENOTDIR}},
		"deep-dangling-fileasdir6": {"dangling-file/g", lookupResult{handlePath: "/dangling-file", remainingPath: "g", fileType: unix.S_IFDIR, err: unix.ENOTDIR}},
		// Symlink loops.
		"loop":        {"loop/link", lookupResult{err: unix.ELOOP}},
		"loop-basic1": {"loop/basic-loop1", lookupResult{err: unix.ELOOP}},
		"loop-basic2": {"loop/basic-loop2", lookupResult{err: unix.ELOOP}},
		"loop-basic3": {"loop/basic-loop3", lookupResult{err: unix.ELOOP}},
	} {
		test := test // copy iterator
		// Update the handlePath to be inside our root.
		if test.expected.handlePath != "" {
			test.expected.handlePath = filepath.Join(root, test.expected.handlePath)
		}
		t.Run(name, func(t *testing.T) {
			checkPartialLookup(t, partialLookupFn, rootDir, test.unsafePath, test.expected)
		})
	}
}

func tRunWrapper(t *testing.T) testutils.TRunFunc {
	return func(name string, doFn testutils.TDoFunc) {
		t.Run(name, func(t *testing.T) {
			doFn(t)
		})
	}
}

func TestPartialLookupInRoot(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		testPartialLookup(t, PartialLookupInRoot)
	})
}

func TestPartialOpenat2(t *testing.T) {
	testPartialLookup(t, partialLookupOpenat2)
}

func TestPartialLookupInRoot_BadInode(t *testing.T) {
	testutils.RequireRoot(t) // mknod

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		partialLookupFn := PartialLookupInRoot

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
			expected   lookupResult
		}{
			// Complete lookups.
			"char-trailing": {"foo/whiteout", lookupResult{handlePath: "/foo/whiteout", remainingPath: "", fileType: unix.S_IFCHR}},
			"blk-trailing":  {"foo/whiteout-blk", lookupResult{handlePath: "/foo/whiteout-blk", remainingPath: "", fileType: unix.S_IFBLK}},
			// Partial lookups due to hitting a non-directory.
			"char-dot":     {"foo/whiteout/.", lookupResult{handlePath: "/foo/whiteout", remainingPath: ".", fileType: unix.S_IFCHR, err: unix.ENOTDIR}},
			"char-dotdot1": {"foo/whiteout/..", lookupResult{handlePath: "/foo/whiteout", remainingPath: "..", fileType: unix.S_IFCHR, err: unix.ENOTDIR}},
			"char-dotdot2": {"foo/whiteout/../foo/bar", lookupResult{handlePath: "/foo/whiteout", remainingPath: "../foo/bar", fileType: unix.S_IFCHR, err: unix.ENOTDIR}},
			"blk-dot":      {"foo/whiteout-blk/.", lookupResult{handlePath: "/foo/whiteout-blk", remainingPath: ".", fileType: unix.S_IFBLK, err: unix.ENOTDIR}},
			"blk-dotdot1":  {"foo/whiteout-blk/..", lookupResult{handlePath: "/foo/whiteout-blk", remainingPath: "..", fileType: unix.S_IFBLK, err: unix.ENOTDIR}},
			"blk-dotdot2":  {"foo/whiteout-blk/../foo/bar", lookupResult{handlePath: "/foo/whiteout-blk", remainingPath: "../foo/bar", fileType: unix.S_IFBLK, err: unix.ENOTDIR}},
		} {
			test := test // copy iterator
			// Update the handlePath to be inside our root.
			if test.expected.handlePath != "" {
				test.expected.handlePath = filepath.Join(root, test.expected.handlePath)
			}
			t.Run(name, func(t *testing.T) {
				checkPartialLookup(t, partialLookupFn, rootDir, test.unsafePath, test.expected)
			})
		}
	})
}

type racingLookupMeta struct {
	pauseCh                                                      chan struct{}
	passOkCount, passErrCount, skipCount, failCount, badErrCount int // test state counts
	badNameCount, fixRemainingPathCount                          int // workaround counts
	skipErrCounts                                                map[error]int
}

func newRacingLookupMeta(pauseCh chan struct{}) *racingLookupMeta {
	return &racingLookupMeta{
		pauseCh:       pauseCh,
		skipErrCounts: map[error]int{},
	}
}

func (m *racingLookupMeta) checkPartialLookup(t *testing.T, rootDir fd.Fd, unsafePath string, skipErrs []error, allowedResults []lookupResult) {
	// Similar to checkPartialLookup, but with extra logic for
	// handling the lookup stopping partly through the lookup.
	handle, remainingPath, err := PartialLookupInRoot(rootDir, unsafePath)
	var (
		handleName string
		realPath   string
		unixStat   unix.Stat_t
	)
	if handle != nil {
		handleName = handle.Name()

		// Get the "proper" name from ProcSelfFdReadlink.
		m.pauseCh <- struct{}{}
		realPath, err = procfs.ProcSelfFdReadlink(handle)
		<-m.pauseCh
		require.NoError(t, err, "get real path of returned handle")

		unixStat, err = fd.Fstat(handle)
		require.NoError(t, err, "stat handle")

		_ = handle.Close()
	} else if err != nil {
		for _, skipErr := range skipErrs {
			if errors.Is(err, skipErr) {
				m.skipErrCounts[skipErr]++
				m.skipCount++
				return
			}
		}
		for _, allowed := range allowedResults {
			if allowed.err != nil && errors.Is(err, allowed.err) {
				m.passErrCount++
				return
			}
		}
		// If we didn't hit any of the allowed errors, it's an
		// unexpected error.
		assert.NoError(t, err)
		m.badErrCount++
		return
	}

	if realPath != handleName {
		// It's possible for handle.Name() to be wrong because while it was
		// correct when it was set, it might not match if the path was swapped
		// afterwards (for both openat2 and PartialLookupInRoot).
		m.badNameCount++
	}

	// It's possible for lookups with ".." components to decide to cut off the
	// lookup partially through the resolution when dealing with a swapping
	// attack, so for the purposes of validating our tests we clean up the
	// remainingPath so that it has all of the ".." components removed (but
	// include this in our statistics).
	fullLogicalPath := filepath.Join(realPath, remainingPath)
	newRemainingPath, err := filepath.Rel(realPath, fullLogicalPath)
	require.NoErrorf(t, err, "clean remaining path %s", remainingPath)
	if remainingPath != newRemainingPath {
		m.fixRemainingPathCount++
	}
	remainingPath = newRemainingPath

	gotResult := lookupResult{
		handlePath:    realPath,
		remainingPath: remainingPath,
		fileType:      unixStat.Mode & unix.S_IFMT,
	}
	counter := &m.passOkCount
	if !assert.Contains(t, allowedResults, gotResult) {
		counter = &m.failCount
	}
	(*counter)++
}

// doRenameExchangeLoop runs in a loop swapping two paths, intended to be run
// in a goroutine during a test.
func doRenameExchangeLoop(pauseCh chan struct{}, exitCh <-chan struct{}, dir fd.Fd, pathA, pathB string) {
	for {
		select {
		case <-exitCh:
			return
		case <-pauseCh:
			// Wait for caller to unpause us.
			select {
			case pauseCh <- struct{}{}:
			case <-exitCh:
				return
			}
		default:
			// Do the swap twice so that we only pause when we are in a
			// "correct" state.
			for i := 0; i < 2; i++ {
				err := unix.Renameat2(int(dir.Fd()), pathA, int(dir.Fd()), pathB, unix.RENAME_EXCHANGE)
				if err != nil && int(dir.Fd()) != -1 && !errors.Is(err, unix.EBADF) {
					// Should never happen, and if it does we will potentially
					// enter a bad filesystem state if we get paused.
					panic(fmt.Sprintf("renameat2([%d]%q, %q, ..., %q, RENAME_EXCHANGE) = %v", int(dir.Fd()), dir.Name(), pathA, pathB, err))
				}
			}
		}
		// Make sure GC doesn't close the directory handle.
		runtime.KeepAlive(dir)
	}
}

func TestPartialLookup_RacingRename(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race tests in short mode")
	}
	testutils.RequireRenameExchange(t)

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		tree := []string{
			"dir a/b/c/d",
			"symlink b-link ../b/../b/../b/../b/../b/../b/../b/../b/../b/../b/../b/../b/../b",
			"symlink c-link ../../b/c/../../b/c/../../b/c/../../b/c/../../b/c/../../b/c/../../b/c/../../b/c/../../b/c",
			"file file",
			"symlink bad-link /foobar",
		}

		var (
			handlePath      = "/a/b/c/d"
			remainingPath   = "e"
			defaultExpected []lookupResult
		)
		// The lookup could stop at any component other than /a, so allow all
		// of them.
		for handlePath != "/" {
			defaultExpected = append(defaultExpected, lookupResult{
				handlePath:    handlePath,
				remainingPath: remainingPath,
				fileType:      unix.S_IFDIR,
			})
			handlePath, remainingPath = filepath.Dir(handlePath), filepath.Join(filepath.Base(handlePath), remainingPath)
		}
		for name, test := range map[string]struct {
			subPathA, subPathB string
			unsafePath         string
			skipErrs           []error
			allowedResults     []lookupResult
		}{
			// Swap a symlink in and out.
			"swap-dir-link1-basic":   {"a/b", "b-link", "a/b/c/d/e", nil, gocompat.SlicesClone(defaultExpected)},
			"swap-dir-link2-basic":   {"a/b/c", "c-link", "a/b/c/d/e", nil, gocompat.SlicesClone(defaultExpected)},
			"swap-dir-link1-dotdot1": {"a/b", "b-link", "a/b/../b/../b/../b/../b/../b/../b/c/d/../d/../d/../d/../d/../d/e", nil, gocompat.SlicesClone(defaultExpected)},
			"swap-dir-link1-dotdot2": {"a/b", "b-link", "a/b/c/../c/../c/../c/../c/../c/../c/d/../d/../d/../d/../d/../d/e", nil, gocompat.SlicesClone(defaultExpected)},
			"swap-dir-link2-dotdot":  {"a/b/c", "c-link", "a/b/c/../c/../c/../c/../c/../c/../c/d/../d/../d/../d/../d/../d/e", nil, gocompat.SlicesClone(defaultExpected)},
			// TODO: Swap a directory.
			// Swap a non-directory.
			"swap-dir-file-basic": {"a/b", "file", "a/b/c/d/e", []error{unix.ENOTDIR, unix.ENOENT}, append(
				// We could hit one of the final paths.
				gocompat.SlicesClone(defaultExpected),
				// We could hit the file and stop resolving.
				lookupResult{handlePath: "/file", remainingPath: "c/d/e", fileType: unix.S_IFREG},
			)},
			"swap-dir-file-dotdot": {"a/b", "file", "a/b/c/../c/../c/../c/../c/../c/../c/d/../d/../d/../d/../d/../d/e", []error{unix.ENOTDIR, unix.ENOENT}, append(
				// We could hit one of the final paths.
				gocompat.SlicesClone(defaultExpected),
				// We could hit the file and stop resolving.
				lookupResult{handlePath: "/file", remainingPath: "c/d/e", fileType: unix.S_IFREG},
			)},
			// Swap a dangling symlink.
			"swap-dir-danglinglink-basic":  {"a/b", "bad-link", "a/b/c/d/e", []error{unix.ENOENT}, gocompat.SlicesClone(defaultExpected)},
			"swap-dir-danglinglink-dotdot": {"a/b", "bad-link", "a/b/c/../c/../c/../c/../c/../c/../c/d/../d/../d/../d/../d/../d/e", []error{unix.ENOENT}, gocompat.SlicesClone(defaultExpected)},
			// Swap the root.
			"swap-root-basic":        {".", "../outsideroot", "a/b/c/d/e", nil, gocompat.SlicesClone(defaultExpected)},
			"swap-root-dotdot":       {".", "../outsideroot", "a/b/../../a/b/../../a/b/../../a/b/../../a/b/../../a/b/../../a/b/../../a/b/c/d/e", nil, gocompat.SlicesClone(defaultExpected)},
			"swap-root-dotdot-extra": {".", "../outsideroot", "a/" + strings.Repeat("b/c/d/../../../", 10) + "b/c/d/e", nil, gocompat.SlicesClone(defaultExpected)},
			// Swap one of our walking paths outside the root.
			"swap-dir-outsideroot-basic": {"a/b", "../outsideroot", "a/b/c/d/e", nil, append(
				// We could hit the expected path.
				gocompat.SlicesClone(defaultExpected),
				// We could also land in the "outsideroot" path. This is okay
				// because there was a moment when this directory was inside
				// the root, and the attacker moved it outside the root. If we
				// were to go into "..", the lookup would've failed (and we
				// would get an error here if that wasn't the case).
				lookupResult{handlePath: "../outsideroot", remainingPath: "c/d/e", fileType: unix.S_IFDIR},
				lookupResult{handlePath: "../outsideroot/c", remainingPath: "d/e", fileType: unix.S_IFDIR},
				lookupResult{handlePath: "../outsideroot/c/d", remainingPath: "e", fileType: unix.S_IFDIR},
			)},
			"swap-dir-outsideroot-dotdot": {"a/b", "../outsideroot", "a/b/../../a/b/../../a/b/../../a/b/../../a/b/../../a/b/../../a/b/../../a/b/c/d/e", nil, append(
				// We could hit the expected path.
				gocompat.SlicesClone(defaultExpected),
				// We could also land in the "outsideroot" path. This is okay
				// because there was a moment when this directory was inside
				// the root, and the attacker moved it outside the root.
				//
				// Neither openat2 nor PartialLookupInRoot will allow us to
				// walk into ".." in this case (escaping the root), and we
				// would catch that if it did happen.
				lookupResult{handlePath: "../outsideroot", remainingPath: "c/d/e", fileType: unix.S_IFDIR},
				lookupResult{handlePath: "../outsideroot/c", remainingPath: "d/e", fileType: unix.S_IFDIR},
				lookupResult{handlePath: "../outsideroot/c/d", remainingPath: "e", fileType: unix.S_IFDIR},
			)},
		} {
			test := test // copy iterator
			test.skipErrs = append(test.skipErrs, unix.EAGAIN, unix.EXDEV)
			t.Run(name, func(t *testing.T) {
				root := testutils.CreateTree(t, tree...)

				// Update the handlePath to be inside our root.
				for idx := range test.allowedResults {
					test.allowedResults[idx].handlePath = filepath.Join(root, test.allowedResults[idx].handlePath)
				}

				// Create an "outsideroot" path as a sibling to our root, for
				// swapping.
				err := os.MkdirAll(filepath.Join(root, "../outsideroot"), 0o755)
				require.NoError(t, err)

				rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
				require.NoError(t, err)
				defer rootDir.Close() //nolint:errcheck // test code

				// If the swapping subpaths are "." we need to use an absolute
				// path because renaming "." isn't allowed.
				for _, subPath := range []*string{&test.subPathA, &test.subPathB} {
					if filepath.Join(root, *subPath) == root {
						*subPath = root
					}
				}

				// Run a goroutine that spams a rename in the root.
				pauseCh := make(chan struct{})
				exitCh := make(chan struct{})
				defer close(exitCh)
				go doRenameExchangeLoop(pauseCh, exitCh, rootDir, test.subPathA, test.subPathB)

				// Do several runs to try to catch bugs.
				const (
					testRuns     = 3000
					minPassCount = 10
				)
				m := newRacingLookupMeta(pauseCh)
				doneRuns := 0
				for ; doneRuns < testRuns || m.passOkCount < minPassCount; doneRuns++ {
					m.checkPartialLookup(t, rootDir, test.unsafePath, test.skipErrs, test.allowedResults)
					// Make sure we don't infinite loop here.
					if doneRuns >= 50*testRuns {
						break
					}
				}

				pct := func(count int) string {
					return fmt.Sprintf("%d(%.3f%%)", count, 100.0*float64(count)/float64(doneRuns))
				}

				// No passing runs is a bit unfortunate, but some of our tests
				// can do that and failing here would just lead to flaky tests.
				if m.passOkCount == 0 {
					t.Logf("WARNING: NO PASSING RUNS!")
				}

				// Output some stats.
				t.Logf("after %d runs: passOk=%s passErr=%s skip=%s fail=%s (+badErr=%s)",
					// runs and breakdown of path-related (pass, fail) as well as skipped runs
					doneRuns, pct(m.passOkCount), pct(m.passErrCount), pct(m.skipCount), pct(m.failCount),
					// failures due to incorrect errors (rather than bad paths)
					pct(m.badErrCount))
				t.Logf("  badHandleName=%s fixRemainingPath=%s",
					// stats for how many test runs had to have some "workarounds"
					pct(m.badNameCount), pct(m.fixRemainingPathCount))
				if len(m.skipErrCounts) > 0 {
					t.Logf("  skipErr breakdown:")
					for err, count := range m.skipErrCounts {
						t.Logf("   %3.d: %v", count, err)
					}
				}
			})
		}
	})
}

type ssOperation interface {
	String() string
	Do(*testing.T, *symlinkStack) error
}

type ssOpPop struct{ part string }

func (op ssOpPop) Do(_ *testing.T, s *symlinkStack) error { return s.PopPart(op.part) }

func (op ssOpPop) String() string { return fmt.Sprintf("PopPart(%q)", op.part) }

type ssOpSwapLink struct {
	part, dirName, expectedPath, linkTarget string
}

func fakeFile(name string) (*os.File, error) {
	fd, err := unix.Open(".", unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: ".", Err: err}
	}
	return os.NewFile(uintptr(fd), name), nil
}

func (op ssOpSwapLink) Do(t *testing.T, s *symlinkStack) error {
	f, err := fakeFile(op.dirName)
	require.NoErrorf(t, err, "make fake file with %q name", op.dirName)
	return s.SwapLink(op.part, f, op.expectedPath, op.linkTarget)
}

func (op ssOpSwapLink) String() string {
	return fmt.Sprintf("SwapLink(%q, <%s>, %q, %q)", op.part, op.dirName, op.expectedPath, op.linkTarget)
}

type ssOp struct {
	op          ssOperation
	expectedErr error
}

func (t ssOp) String() string { return fmt.Sprintf("%s = %v", t.op, t.expectedErr) }

func dumpStack(t *testing.T, ss symlinkStack) {
	for i, sym := range ss {
		t.Logf("ss[%d] %s", i, sym)
	}
}

func testSymlinkStack(t *testing.T, ops ...ssOp) symlinkStack {
	var ss symlinkStack
	for _, op := range ops {
		err := op.op.Do(t, &ss)
		if !assert.ErrorIsf(t, err, op.expectedErr, "%s", op) { //nolint:testifylint
			dumpStack(t, ss)
			ss.Close()
			t.FailNow()
		}
	}
	return ss
}

func TestSymlinkStackBasic(t *testing.T) {
	ss := testSymlinkStack(t,
		ssOp{op: ssOpSwapLink{"foo", "A", "", "bar/baz"}},
		ssOp{op: ssOpSwapLink{"bar", "B", "baz", "abcd"}},
		ssOp{op: ssOpPop{"abcd"}},
		ssOp{op: ssOpSwapLink{"baz", "C", "", "taillink"}},
		ssOp{op: ssOpPop{"taillink"}},
		ssOp{op: ssOpPop{"anotherbit"}},
	)
	defer ss.Close() //nolint:errcheck // test code

	if !assert.True(t, ss.IsEmpty()) {
		dumpStack(t, ss)
		t.FailNow()
	}
}

func TestSymlinkStackBadPop(t *testing.T) {
	ss := testSymlinkStack(t,
		ssOp{op: ssOpSwapLink{"foo", "A", "", "bar/baz"}},
		ssOp{op: ssOpSwapLink{"bar", "B", "baz", "abcd"}},
		ssOp{op: ssOpSwapLink{"bad", "C", "", "abcd"}, expectedErr: errBrokenSymlinkStack},
		ssOp{op: ssOpPop{"abcd"}},
		ssOp{op: ssOpSwapLink{"baz", "C", "", "abcd"}},
		ssOp{op: ssOpSwapLink{"abcd", "D", "", ""}}, // TODO: This is technically an invalid thing to push.
		ssOp{op: ssOpSwapLink{"another", "E", "", ""}, expectedErr: errBrokenSymlinkStack},
	)
	defer ss.Close() //nolint:errcheck // test code
}

type expectedStackEntry struct {
	expectedDirName  string
	expectedUnwalked []string
}

func testStackContents(t *testing.T, msg string, ss symlinkStack, expected ...expectedStackEntry) {
	if len(expected) > 0 {
		require.Lenf(t, ss, len(expected), "%s: stack should be the expected length", msg)
		require.Falsef(t, ss.IsEmpty(), "%s: stack IsEmpty should be false", msg)
	} else {
		require.Emptyf(t, ss, "%s: stack should be empty", msg)
		require.Truef(t, ss.IsEmpty(), "%s: stack IsEmpty should be true", msg)
	}

	for idx, entry := range expected {
		assert.Equalf(t, entry.expectedDirName, ss[idx].dir.Name(), "%s: stack entry %d name mismatch", msg, idx)
		if len(entry.expectedUnwalked) > 0 {
			assert.Equalf(t, entry.expectedUnwalked, ss[idx].linkUnwalked, "%s: stack entry %d unwalked link entries mismatch", msg, idx)
		} else {
			assert.Emptyf(t, ss[idx].linkUnwalked, "%s: stack entry %d unwalked link entries", msg, idx)
		}
	}

	// Fail the test immediately so we can get the current stack in the test output.
	if t.Failed() {
		t.FailNow()
	}
}

func TestSymlinkStackBasicTailChain(t *testing.T) {
	ss := testSymlinkStack(t,
		ssOp{op: ssOpSwapLink{"foo", "A", "", "tailA"}},
		ssOp{op: ssOpSwapLink{"tailA", "B", "", "tailB"}},
		ssOp{op: ssOpSwapLink{"tailB", "C", "", "tailC"}},
		ssOp{op: ssOpSwapLink{"tailC", "D", "", "tailD"}},
		ssOp{op: ssOpSwapLink{"tailD", "E", "", "foo/taillink"}},
	)
	defer func() {
		if t.Failed() {
			dumpStack(t, ss)
		}
	}()
	defer ss.Close() //nolint:errcheck // test code

	// Basic expected contents.
	testStackContents(t, "initial state", ss,
		// The top 4 entries should have no unwalked links.
		expectedStackEntry{"A", nil},
		expectedStackEntry{"B", nil},
		expectedStackEntry{"C", nil},
		expectedStackEntry{"D", nil},
		// And the final entry should just be foo/taillink.
		expectedStackEntry{"E", []string{"foo", "taillink"}},
	)

	// Popping "foo" should keep the tail-chain.
	require.NoError(t, ss.PopPart("foo"), "pop foo")
	testStackContents(t, "pop tail-chain end", ss,
		// The top 4 entries should have no unwalked links.
		expectedStackEntry{"A", nil},
		expectedStackEntry{"B", nil},
		expectedStackEntry{"C", nil},
		expectedStackEntry{"D", nil},
		// And the final entry should just be foo/taillink.
		expectedStackEntry{"E", []string{"taillink"}},
	)

	// Dropping taillink should empty the stack.
	require.NoError(t, ss.PopPart("taillink"), "pop taillink")
	testStackContents(t, "pop last element in tail-chain", ss)
	assert.True(t, ss.IsEmpty(), "pop last element in tail-chain should empty chain")
}

func TestSymlinkStackTailChain(t *testing.T) {
	ss := testSymlinkStack(t,
		ssOp{op: ssOpSwapLink{"foo", "A", "", "tailA/subdir1"}},
		// First tail-chain.
		ssOp{op: ssOpSwapLink{"tailA", "B", "", "tailB"}},
		ssOp{op: ssOpSwapLink{"tailB", "C", "", "tailC"}},
		ssOp{op: ssOpSwapLink{"tailC", "D", "", "tailD"}},
		ssOp{op: ssOpSwapLink{"tailD", "E", "", "taillink1/subdir2"}},
		// Second tail-chain.
		ssOp{op: ssOpSwapLink{"taillink1", "F", "", "tailE"}},
		ssOp{op: ssOpSwapLink{"tailE", "G", "", "tailF"}},
		ssOp{op: ssOpSwapLink{"tailF", "H", "", "tailG"}},
		ssOp{op: ssOpSwapLink{"tailG", "I", "", "tailH"}},
		ssOp{op: ssOpSwapLink{"tailH", "J", "", "tailI"}},
		ssOp{op: ssOpSwapLink{"tailI", "K", "", "taillink2/.."}},
	)
	defer func() {
		if t.Failed() {
			dumpStack(t, ss)
		}
	}()
	defer ss.Close() //nolint:errcheck // test code

	// Basic expected contents.
	initialState := []expectedStackEntry{
		// Top entry is not a tail-chain.
		{"A", []string{"subdir1"}},
		// The first tail-chain should have no unwalked links.
		{"B", nil},
		{"C", nil},
		{"D", nil},
		// Final entry in the first tail-chain.
		{"E", []string{"subdir2"}},
		// The second tail-chain should have no unwalked links.
		{"F", nil},
		{"G", nil},
		{"H", nil},
		{"I", nil},
		{"J", nil},
		// Final entry in the second tail-chain.
		{"K", []string{"taillink2", ".."}},
	}

	testStackContents(t, "initial state", ss, initialState...)

	// Trying to pop "." does nothing.
	for i := 0; i < 20; i++ {
		require.NoError(t, ss.PopPart("."), `popping "." should never fail`)
		// NOTE: Same contents as above.
		testStackContents(t, "noop pop .", ss, initialState...)
	}

	// Popping any of the early tail chain entries must fail.
	for _, badPart := range []string{"subdir1", "subdir2", ".."} {
		require.ErrorIsf(t, ss.PopPart(badPart), errBrokenSymlinkStack, "bad pop %q", badPart)
		// NOTE: Same contents as above.
		testStackContents(t, "bad pop "+badPart, ss, initialState...)
	}

	// Dropping the second-last entry should keep the tail-chain.
	require.NoError(t, ss.PopPart("taillink2"), "pop taillink2")
	testStackContents(t, "pop non-last element in second tail-chain", ss,
		// Top entry is not a tail-chain.
		expectedStackEntry{"A", []string{"subdir1"}},
		// The first tail-chain should have no unwalked links.
		expectedStackEntry{"B", nil},
		expectedStackEntry{"C", nil},
		expectedStackEntry{"D", nil},
		// Final entry in the first tail-chain.
		expectedStackEntry{"E", []string{"subdir2"}},
		// The second tail-chain should have no unwalked links.
		expectedStackEntry{"F", nil},
		expectedStackEntry{"G", nil},
		expectedStackEntry{"H", nil},
		expectedStackEntry{"I", nil},
		expectedStackEntry{"J", nil},
		// Final entry in the second tail-chain.
		expectedStackEntry{"K", []string{".."}},
	)

	// Dropping the last entry should only drop the final tail-chain.
	require.NoError(t, ss.PopPart(".."), "pop ..")
	testStackContents(t, "pop last element in second tail-chain", ss,
		// Top entry is not a tail-chain.
		expectedStackEntry{"A", []string{"subdir1"}},
		// The first tail-chain should have no unwalked links.
		expectedStackEntry{"B", nil},
		expectedStackEntry{"C", nil},
		expectedStackEntry{"D", nil},
		// Final entry in the first tail-chain.
		expectedStackEntry{"E", []string{"subdir2"}},
	)

	// Dropping the last entry should only drop the tail-chain.
	require.NoError(t, ss.PopPart("subdir2"), "pop subdir2")
	testStackContents(t, "pop last element in first tail-chain", ss,
		// Top entry is not a tail-chain.
		expectedStackEntry{"A", []string{"subdir1"}},
	)

	// Dropping the last entry should empty the stack.
	require.NoError(t, ss.PopPart("subdir1"), "pop subdir1")
	testStackContents(t, "pop last element", ss)
	assert.True(t, ss.IsEmpty(), "pop last element should empty stack")
}
