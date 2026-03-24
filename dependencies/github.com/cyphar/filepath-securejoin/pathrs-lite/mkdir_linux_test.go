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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/gopathrs"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/procfs"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/testutils"
)

type mkdirAllFunc func(t *testing.T, root, unsafePath string, mode os.FileMode) error

var mkdirAll_MkdirAll mkdirAllFunc = func(_ *testing.T, root, unsafePath string, mode os.FileMode) error { //nolint:revive // underscores are more readable for test helpers
	// We can't check expectedPath here.
	return pathrs.MkdirAll(root, unsafePath, mode)
}

var mkdirAll_MkdirAllHandle mkdirAllFunc = func(t *testing.T, root, unsafePath string, mode os.FileMode) error { //nolint:revive // underscores are more readable for test helpers
	// Same logic as MkdirAll.
	rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer rootDir.Close() //nolint:errcheck // test code
	handle, err := pathrs.MkdirAllHandle(rootDir, unsafePath, mode)
	if err != nil {
		return err
	}
	defer handle.Close() //nolint:errcheck // test code

	// We can lookup the expected path again to get the full path. This will
	// give a reasonable result because we aren't being attacked in this
	// particular test.
	handle2, err := pathrs.OpenatInRoot(rootDir, unsafePath)
	require.NoError(t, err)
	expectedPath, err := procfs.ProcSelfFdReadlink(handle2)
	require.NoError(t, err)

	// Now double-check that the handle is correct.
	gotPath, err := procfs.ProcSelfFdReadlink(handle)
	require.NoError(t, err, "get real path of returned handle")
	assert.Equal(t, expectedPath, gotPath, "wrong final path from MkdirAllHandle")
	// Also check that the f.Name() is correct while we're at it (this is
	// not always guaranteed but it's better to try at least).
	assert.Equal(t, expectedPath, handle.Name(), "handle from MkdirAllHandle has the wrong .Name()")
	return nil
}

func checkMkdirAll(t *testing.T, mkdirAll mkdirAllFunc, root, unsafePath string, mode os.FileMode, expectedMode int, expectedErr error) {
	rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	require.NoError(t, err)
	defer rootDir.Close() //nolint:errcheck // test code

	// Before trying to make the tree, figure out what components don't exist
	// yet so we can check them later.
	handle, remainingPath, err := gopathrs.PartialLookupInRoot(rootDir, unsafePath)
	handleName := "<nil>"
	if handle != nil {
		handleName = handle.Name()
		defer handle.Close() //nolint:errcheck // test code
	}
	defer func() {
		if t.Failed() {
			t.Logf("PartialLookupInRoot(%s, %s) -> (<%s>, %s, %v)", root, unsafePath, handleName, remainingPath, err)
		}
	}()

	// Actually make the tree.
	err = mkdirAll(t, root, unsafePath, mode)
	require.ErrorIsf(t, err, expectedErr, "MkdirAll(%q, %q)", root, unsafePath)

	remainingPath = filepath.Join("/", remainingPath)
	for remainingPath != filepath.Dir(remainingPath) {
		stat, err := fd.Fstatat(handle, "./"+remainingPath, unix.AT_SYMLINK_NOFOLLOW)
		if expectedErr == nil {
			// Check that the new components have the right mode.
			if assert.NoErrorf(t, err, "unexpected error when checking new directory %q", remainingPath) {
				assert.Equalf(t, uint32(unix.S_IFDIR|expectedMode), stat.Mode, "new directory %q has the wrong mode", remainingPath)
			}
		} else {
			// Check that none of the components are directories (i.e. make
			// sure that the MkdirAll was a no-op).
			if err == nil {
				assert.NotEqualf(t, uint32(unix.S_IFDIR), stat.Mode&unix.S_IFMT, "failed MkdirAll created a new directory at %q", remainingPath)
			}
		}
		// Jump up a level.
		remainingPath = filepath.Dir(remainingPath)
	}
}

func testMkdirAll_Basic(t *testing.T, mkdirAll mkdirAllFunc) { //nolint:revive // underscores are more readable for test helpers
	// We create a new tree for each test, but the template is the same.
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
		// Symlink loop.
		"dir loop",
		"symlink loop/link ../loop/link",
		// S_ISGID directory.
		"dir sgid-self ::2755",
		"dir sgid-sticky-self ::3755",
	}

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		for name, test := range map[string]struct {
			unsafePath       string
			expectedErr      error
			expectedModeBits int
		}{
			"existing":              {unsafePath: "a"},
			"basic":                 {unsafePath: "a/b/c/d/e/f/g/h/i/j"},
			"dotdot-in-nonexisting": {unsafePath: "a/b/c/d/e/f/g/h/i/j/k/../lmnop", expectedErr: unix.ENOENT},
			"dotdot-in-existing":    {unsafePath: "b/c/../c/./d/e/f/g/h"},
			"dotdot-after-symlink":  {unsafePath: "e/../dd/ee/ff"},
			// Check that trying to create under a file fails.
			"nondir-trailing":         {unsafePath: "b/c/file", expectedErr: unix.ENOTDIR},
			"nondir-dotdot":           {unsafePath: "b/c/file/../d", expectedErr: unix.ENOTDIR},
			"nondir-subdir":           {unsafePath: "b/c/file/subdir", expectedErr: unix.ENOTDIR},
			"nondir-symlink-trailing": {unsafePath: "b-file", expectedErr: unix.ENOTDIR},
			"nondir-symlink-dotdot":   {unsafePath: "b-file/../d", expectedErr: unix.ENOTDIR},
			"nondir-symlink-subdir":   {unsafePath: "b-file/subdir", expectedErr: unix.ENOTDIR},
			// Dangling symlinks are not followed.
			"dangling1-trailing": {unsafePath: "a-fake1", expectedErr: unix.ENOTDIR},
			"dangling1-basic":    {unsafePath: "a-fake1/foo", expectedErr: unix.ENOTDIR},
			"dangling1-dotdot":   {unsafePath: "a-fake1/../bar/baz", expectedErr: unix.ENOENT},
			"dangling2-trailing": {unsafePath: "a-fake2", expectedErr: unix.ENOTDIR},
			"dangling2-basic":    {unsafePath: "a-fake2/foo", expectedErr: unix.ENOTDIR},
			"dangling2-dotdot":   {unsafePath: "a-fake2/../bar/baz", expectedErr: unix.ENOENT},
			"dangling3-trailing": {unsafePath: "a-fake3", expectedErr: unix.ENOTDIR},
			"dangling3-basic":    {unsafePath: "a-fake3/foo", expectedErr: unix.ENOTDIR},
			"dangling3-dotdot":   {unsafePath: "a-fake3/../bar/baz", expectedErr: unix.ENOENT},
			// Non-lexical symlinks should work.
			"nonlexical-basic":           {unsafePath: "target/foo"},
			"nonlexical-level1-abs":      {unsafePath: "link1/target_abs/foo"},
			"nonlexical-level1-rel":      {unsafePath: "link1/target_rel/foo"},
			"nonlexical-level2-abs-abs":  {unsafePath: "link2/link1_abs/target_abs/foo"},
			"nonlexical-level2-abs-rel":  {unsafePath: "link2/link1_abs/target_rel/foo"},
			"nonlexical-level2-abs-open": {unsafePath: "link2/link1_abs/../target/foo"},
			"nonlexical-level2-rel-abs":  {unsafePath: "link2/link1_rel/target_abs/foo"},
			"nonlexical-level2-rel-rel":  {unsafePath: "link2/link1_rel/target_rel/foo"},
			"nonlexical-level2-rel-open": {unsafePath: "link2/link1_rel/../target/foo"},
			"nonlexical-level3-abs":      {unsafePath: "link3/target_abs/foo"},
			"nonlexical-level3-rel":      {unsafePath: "link3/target_rel/foo"},
			// But really tricky dangling symlinks should fail.
			"dangling-tricky1-trailing": {unsafePath: "link3/deep_dangling1", expectedErr: unix.ENOTDIR},
			"dangling-tricky1-basic":    {unsafePath: "link3/deep_dangling1/foo", expectedErr: unix.ENOTDIR},
			"dangling-tricky1-dotdot":   {unsafePath: "link3/deep_dangling1/../bar", expectedErr: unix.ENOENT},
			"dangling-tricky2-trailing": {unsafePath: "link3/deep_dangling2", expectedErr: unix.ENOTDIR},
			"dangling-tricky2-basic":    {unsafePath: "link3/deep_dangling2/foo", expectedErr: unix.ENOTDIR},
			"dangling-tricky2-dotdot":   {unsafePath: "link3/deep_dangling2/../bar", expectedErr: unix.ENOENT},
			// And trying to mkdir inside a loop should fail.
			"loop-trailing": {unsafePath: "loop/link", expectedErr: unix.ELOOP},
			"loop-basic":    {unsafePath: "loop/link/foo", expectedErr: unix.ELOOP},
			"loop-dotdot":   {unsafePath: "loop/link/../foo", expectedErr: unix.ELOOP},
			// Make sure the S_ISGID handling is correct.
			"sgid-dir-ownedbyus":        {unsafePath: "sgid-self/foo/bar/baz", expectedModeBits: unix.S_ISGID},
			"sgid-sticky-dir-ownedbyus": {unsafePath: "sgid-sticky-self/foo/bar/baz", expectedModeBits: unix.S_ISGID},
		} {
			test := test // copy iterator
			t.Run(name, func(t *testing.T) {
				root := testutils.CreateTree(t, tree...)
				const mode = 0o711
				checkMkdirAll(t, mkdirAll, root, test.unsafePath, mode, test.expectedModeBits|mode, test.expectedErr)
			})
		}
	})
}

func TestMkdirAll_Basic(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	testMkdirAll_Basic(t, mkdirAll_MkdirAll)
}

func TestMkdirAllHandle_Basic(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	testMkdirAll_Basic(t, mkdirAll_MkdirAllHandle)
}

func TestMkdirAll_BadRoot(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	t.Run("MkdirAll", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "does/not/exist")

		err := pathrs.MkdirAll(root, "foo/bar", 0o755)
		require.ErrorIs(t, err, os.ErrNotExist, "MkdirAll with bad root")
	})
	// TODO: Should we add checks for nil *os.File?
}

func testMkdirAll_AsRoot(t *testing.T, mkdirAll mkdirAllFunc) { //nolint:revive // underscores are more readable for test helpers
	testutils.RequireRoot(t) // chown

	// We create a new tree for each test, but the template is the same.
	tree := []string{
		// S_ISGID directories.
		"dir sgid-self ::2755",
		"dir sgid-other 1000:1000:2755",
		"dir sgid-sticky-self ::3755",
		"dir sgid-sticky-other 1000:1000:3755",
	}

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		for name, test := range map[string]struct {
			unsafePath       string
			expectedErr      error
			expectedModeBits int
		}{
			// Make sure the S_ISGID handling is correct.
			"sgid-dir-ownedbyus":           {unsafePath: "sgid-self/foo/bar/baz", expectedModeBits: unix.S_ISGID},
			"sgid-dir-ownedbyother":        {unsafePath: "sgid-other/foo/bar/baz", expectedModeBits: unix.S_ISGID},
			"sgid-sticky-dir-ownedbyus":    {unsafePath: "sgid-sticky-self/foo/bar/baz", expectedModeBits: unix.S_ISGID},
			"sgid-sticky-dir-ownedbyother": {unsafePath: "sgid-sticky-other/foo/bar/baz", expectedModeBits: unix.S_ISGID},
		} {
			test := test // copy iterator
			t.Run(name, func(t *testing.T) {
				root := testutils.CreateTree(t, tree...)
				const mode = 0o711
				checkMkdirAll(t, mkdirAll, root, test.unsafePath, mode, test.expectedModeBits|mode, test.expectedErr)
			})
		}
	})
}

func TestMkdirAll_AsRoot(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	testMkdirAll_AsRoot(t, mkdirAll_MkdirAll)
}

func TestMkdirAllHandle_AsRoot(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	testMkdirAll_AsRoot(t, mkdirAll_MkdirAllHandle)
}

type racingMkdirMeta struct {
	passOkCount, passErrCount, failCount int
	passErrCounts                        map[error]int
}

func newRacingMkdirMeta() *racingMkdirMeta {
	return &racingMkdirMeta{
		passErrCounts: map[error]int{},
	}
}

func (m *racingMkdirMeta) checkMkdirAllHandle_Racing(t *testing.T, root, unsafePath string, mode os.FileMode, allowedErrs []error) { //nolint:revive // underscores are more readable for test helpers
	rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if !assert.NoError(t, err, "open root") { //nolint:testifylint // cannot use require.* in goroutines
		m.failCount++
		return
	}
	defer rootDir.Close() //nolint:errcheck // test code

	handle, err := pathrs.MkdirAllHandle(rootDir, unsafePath, mode)
	if err != nil {
		for _, allowedErr := range allowedErrs {
			if errors.Is(err, allowedErr) {
				m.passErrCounts[allowedErr]++
				m.passErrCount++
				return
			}
		}
		assert.NoError(t, err)
		m.failCount++
		return
	}
	defer handle.Close() //nolint:errcheck // test code

	// It's possible for an attacker to have swapped the final directory, but
	// this is okay because MkdirAll will use pre-existing directories anyway.
	// So there's no need to check the returned handle.
	// TODO: Does it make sense to even try to check the handle path?
	m.passOkCount++
}

func TestMkdirAllHandle_RacingRename(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	if testing.Short() {
		t.Skip("skipping race tests in short mode")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		treeSpec := []string{
			"dir target/a/b/c",
			"dir swapdir-empty-ok ::0711",
			"dir swapdir-empty-badmode ::0777",
			"dir swapdir-nonempty1 ::0711",
			"file swapdir-nonempty1/aaa",
			"dir swapdir-nonempty2 ::0711",
			"dir swapdir-nonempty2/f ::0711",
			"file swapfile foobar ::0711",
		}

		type test struct {
			name         string
			pathA, pathB string
			unsafePath   string
			allowedErrs  []error
		}

		tests := []test{
			{"good", "target/a/b/c/d/e", "swapdir-empty-ok", "target/a/b/c/d/e/f/g/h/i/j/k", nil},
			{"trailing", "target/a/b/c/d/e", "swapdir-empty-badmode", "target/a/b/c/d/e", nil},
			{"partial", "target/a/b/c/d/e", "swapdir-empty-badmode", "target/a/b/c/d/e/f/g/h/i/j/k", nil},
			{"trailing", "target/a/b/c/d/e", "swapdir-nonempty1", "target/a/b/c/d/e", nil},
			{"partial", "target/a/b/c/d/e", "swapdir-nonempty1", "target/a/b/c/d/e/f/g/h/i/j/k", nil},
			{"trailing", "target/a/b/c/d/e", "swapdir-nonempty2", "target/a/b/c/d/e", nil},
			{"partial", "target/a/b/c/d/e", "swapdir-nonempty2", "target/a/b/c/d/e/f/g/h/i/j/k", []error{unix.ENOTDIR}},
			{"trailing", "target/a/b/c/d/e", "swapfile", "target/a/b/c/d/e", []error{unix.ENOTDIR}},
			{"partial", "target/a/b/c/d/e", "swapfile", "target/a/b/c/d/e/f/g/h/i/j/k", []error{unix.ENOTDIR}},
		}

		if unix.Geteuid() == 0 {
			// Add some wrong-uid cases if we are root.
			treeSpec = append(treeSpec,
				"dir swapdir-empty-badowner1 123:0:0711",
				"dir swapdir-empty-badowner2 0:456:0711",
				"dir swapdir-empty-badowner3 111:222:0711",
			)
			tests = append(tests, []test{
				{"trailing", "target/a/b/c/d/e", "swapdir-empty-badowner1", "target/a/b/c/d/e", nil},
				{"partial", "target/a/b/c/d/e", "swapdir-empty-badowner1", "target/a/b/c/d/e/f/g/h/i/j/k", nil},
				{"trailing", "target/a/b/c/d/e", "swapdir-empty-badowner2", "target/a/b/c/d/e", nil},
				{"partial", "target/a/b/c/d/e", "swapdir-empty-badowner2", "target/a/b/c/d/e/f/g/h/i/j/k", nil},
				{"trailing", "target/a/b/c/d/e", "swapdir-empty-badowner3", "target/a/b/c/d/e", nil},
				{"partial", "target/a/b/c/d/e", "swapdir-empty-badowner3", "target/a/b/c/d/e/f/g/h/i/j/k", nil},
			}...)
		}

		for _, test := range tests {
			test := test // copy iterator
			t.Run(fmt.Sprintf("%s-%s", test.pathB, test.name), func(t *testing.T) {
				rootCh := make(chan string)
				defer close(rootCh)
				go func(rootCh <-chan string) {
					var root string
					for {
						select {
						case newRoot, ok := <-rootCh:
							if !ok {
								return
							}
							root = newRoot
						default:
							if root != "" {
								pathA := filepath.Join(root, test.pathA)
								pathB := filepath.Join(root, test.pathB)
								_ = unix.Renameat2(unix.AT_FDCWD, pathA, unix.AT_FDCWD, pathB, unix.RENAME_EXCHANGE)
							}
						}
					}
				}(rootCh)

				// Do several runs to try to catch bugs.
				const (
					testRuns     = 800
					minPassCount = 10
				)
				m := newRacingMkdirMeta()
				doneRuns := 0
				for ; doneRuns < testRuns || m.passOkCount < minPassCount; doneRuns++ {
					root := testutils.CreateTree(t, treeSpec...)

					rootCh <- root
					runtime.Gosched() // give the thread some time to do a rename
					m.checkMkdirAllHandle_Racing(t, root, test.unsafePath, 0o711, test.allowedErrs)
					rootCh <- ""

					// Clean up the root after each run so we don't exhaust all
					// space in the tmpfs.
					_ = os.RemoveAll(root)

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
				t.Logf("after %d runs: passOk=%s passErr=%s fail=%s",
					doneRuns, pct(m.passOkCount), pct(m.passErrCount), pct(m.failCount))
				if len(m.passErrCounts) > 0 {
					t.Logf("  passErr breakdown:")
					for err, count := range m.passErrCounts {
						t.Logf("   %3.d: %v", count, err)
					}
				}
			})
		}
	})
}

func TestMkdirAllHandle_RacingDelete(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	if testing.Short() {
		t.Skip("skipping race tests in short mode")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		treeSpec := []string{
			"dir target/a/b/c",
		}

		for _, test := range []struct {
			name        string
			rmPath      string
			unsafePath  string
			allowedErrs []error
		}{
			{"rm-top", "target", "target/a/b/c/d/e/f/g/h/i/j/k", []error{internal.ErrInvalidDirectory, unix.ENOENT}},
			{"rm-existing", "target/a/b/c", "target/a/b/c/d/e/f/g/h/i/j/k", []error{internal.ErrInvalidDirectory, unix.ENOENT}},
			{"rm-nonexisting", "target/a/b/c/d/e", "target/a/b/c/d/e/f/g/h/i/j/k", []error{internal.ErrInvalidDirectory, unix.ENOENT}},
		} {
			test := test // copy iterator
			t.Run(test.rmPath, func(t *testing.T) {
				rootCh := make(chan string)
				defer close(rootCh)
				go func(rootCh <-chan string) {
					var root string
					for {
						select {
						case newRoot, ok := <-rootCh:
							if !ok {
								return
							}
							root = newRoot
						default:
							if root != "" {
								_ = os.RemoveAll(filepath.Join(root, test.rmPath))
							}
						}
					}
				}(rootCh)

				// Do several runs to try to catch bugs.
				const (
					testRuns     = 800
					minPassCount = 10
				)
				m := newRacingMkdirMeta()
				doneRuns := 0
				for ; doneRuns < testRuns; doneRuns++ {
					root := testutils.CreateTree(t, treeSpec...)

					rootCh <- root
					m.checkMkdirAllHandle_Racing(t, root, test.unsafePath, 0o711, test.allowedErrs)
					rootCh <- ""

					// Clean up the root after each run so we don't exhaust all
					// space in the tmpfs.
					_ = os.RemoveAll(root + "/..")

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
				t.Logf("after %d runs: passOk=%s passErr=%s fail=%s",
					doneRuns, pct(m.passOkCount), pct(m.passErrCount), pct(m.failCount))
				if len(m.passErrCounts) > 0 {
					t.Logf("  passErr breakdown:")
					for err, count := range m.passErrCounts {
						t.Logf("   %3.d: %v", count, err)
					}
				}
			})
		}
	})
}

// Regression test for <https://github.com/opencontainers/runc/issues/4543>.
func TestMkdirAllHandle_RacingCreate(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	if testing.Short() {
		t.Skip("skipping race tests in short mode")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		threadRanges := []int{2, 4, 8, 16, 32, 64, 128, 512, 1024}
		for _, numThreads := range threadRanges {
			numThreads := numThreads
			t.Run(fmt.Sprintf("threads=%d", numThreads), func(t *testing.T) {
				// Do several runs to try to catch bugs.
				const testRuns = 500
				m := newRacingMkdirMeta()
				for i := 0; i < testRuns; i++ {
					root := t.TempDir()
					unsafePath := "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/x/y/z"

					// Spawn many threads that will race against each other to
					// create the same directory.
					startCh := make(chan struct{})
					var finishedWg sync.WaitGroup
					for i := 0; i < numThreads; i++ {
						finishedWg.Add(1)
						go func() {
							<-startCh
							m.checkMkdirAllHandle_Racing(t, root, unsafePath, 0o711, nil)
							finishedWg.Done()
						}()
					}

					// Start all of the threads at the same time.
					close(startCh)

					// Wait for all of the racing threads to finish.
					finishedWg.Wait()

					// Clean up the root after each run so we don't exhaust all
					// space in the tmpfs.
					_ = os.RemoveAll(root)
				}
			})
		}
	})
}
