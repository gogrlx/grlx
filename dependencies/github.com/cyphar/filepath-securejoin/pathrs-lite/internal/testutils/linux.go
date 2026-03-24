// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package testutils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/linux"
)

// RequireRoot skips the current test if we are not root.
func RequireRoot(t TestingT) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root")
	}
}

// RequireRenameExchange skips the current test if renameat2(2) is not
// supported on the running system.
func RequireRenameExchange(t TestingT) {
	err := unix.Renameat2(unix.AT_FDCWD, ".", unix.AT_FDCWD, ".", unix.RENAME_EXCHANGE)
	if errors.Is(err, unix.ENOSYS) {
		t.Skip("test requires RENAME_EXCHANGE support")
	}
}

// TDoFunc is effectively a func(t *testing.T) function but using the
// [TestingT] interface to allow us to write testutils with non-test code. The
// argument is virtually guaranteed to be a *testing.T instance so you can just
// do a type assertion in the body of the closure.
type TDoFunc func(ti TestingT)

// TRunFunc is a wrapper around t.Run but done with an interface that can be
// used in non-testing code. To use this, you should just define a wrapper
// function like this:
//
//	func tRunWrapper(t *testing.T) testutils.TRunFunc {
//		return func(name string, doFn testutils.TDoFunc) {
//			t.Run(name, func(t *testing.T) {
//				doFn(t)
//			})
//		}
//	 }
//
// and then use it with [WithWithoutOpenat2] like so:
//
//	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
//		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
//		/* test code */
//	})
type TRunFunc func(name string, doFn TDoFunc)

// WithWithoutOpenat2 runs a given test with and without openat2 (by forcefully
// disabling its usage).
func WithWithoutOpenat2(doAuto bool, tRunFn TRunFunc, doFn TDoFunc) {
	if doAuto {
		tRunFn("openat2=auto", doFn)
	}
	for _, useOpenat2 := range []bool{true, false} {
		useOpenat2 := useOpenat2 // copy iterator
		tRunFn(fmt.Sprintf("openat2=%v", useOpenat2), func(t TestingT) {
			if useOpenat2 && !linux.HasOpenat2() {
				t.Skip("no openat2 support")
			}

			origHasOpenat2 := linux.HasOpenat2
			linux.HasOpenat2 = func() bool { return useOpenat2 }
			defer func() { linux.HasOpenat2 = origHasOpenat2 }()

			if !useOpenat2 {
				origOpenat2 := fd.Openat2
				fd.Openat2 = func(_ fd.Fd, _ string, _ *unix.OpenHow) (*os.File, error) {
					return nil, fmt.Errorf("INTERNAL ERROR THAT SHOULD NEVER BE SEEN: %w", unix.ENOSYS)
				}
				defer func() { fd.Openat2 = origOpenat2 }()
			}

			doFn(t)
		})
	}
}

// CreateInTree creates a given inode inside the root directory.
//
// Format:
//
//	dir <name> <?uid:gid:mode>
//	file <name> <?content> <?uid:gid:mode>
//	symlink <name> <target>
//	char <name> <major> <minor> <?uid:gid:mode>
//	block <name> <major> <minor> <?uid:gid:mode>
//	fifo <name> <?uid:gid:mode>
//	sock <name> <?uid:gid:mode>
func CreateInTree(t TestingT, root, spec string) {
	f := strings.Fields(spec)
	if len(f) < 2 {
		t.Fatalf("invalid spec %q", spec)
	}
	inoType, subPath, f := f[0], f[1], f[2:]
	fullPath := filepath.Join(root, subPath)

	var setOwnerMode *string
	switch inoType {
	case "dir":
		if len(f) >= 1 {
			setOwnerMode = &f[0]
		}
		MkdirAll(t, fullPath, 0o755)
	case "file":
		var contents []byte
		if len(f) >= 1 {
			contents = []byte(f[0])
		}
		if len(f) >= 2 {
			setOwnerMode = &f[1]
		}
		WriteFile(t, fullPath, contents, 0o644)
	case "symlink":
		if len(f) < 1 {
			t.Fatalf("invalid spec %q", spec)
		}
		target := f[0]
		Symlink(t, target, fullPath)
	case "char", "block":
		if len(f) < 2 {
			t.Fatalf("invalid spec %q", spec)
		}
		if len(f) >= 3 {
			setOwnerMode = &f[2]
		}

		major, err := strconv.Atoi(f[0])
		require.NoErrorf(t, err, "mknod %s: parse major", subPath)
		minor, err := strconv.Atoi(f[1])
		require.NoErrorf(t, err, "mknod %s: parse minor", subPath)
		dev := unix.Mkdev(uint32(major), uint32(minor))

		var mode uint32 = 0o644
		switch inoType {
		case "char":
			mode |= unix.S_IFCHR
		case "block":
			mode |= unix.S_IFBLK
		}
		err = unix.Mknod(fullPath, mode, int(dev))
		require.NoErrorf(t, err, "mknod (%s %d:%d) %s", inoType, major, minor, fullPath)
	case "fifo", "sock":
		if len(f) >= 1 {
			setOwnerMode = &f[0]
		}
		var mode uint32 = 0o644
		switch inoType {
		case "fifo":
			mode |= unix.S_IFIFO
		case "sock":
			mode |= unix.S_IFSOCK
		}
		err := unix.Mknod(fullPath, mode, 0)
		require.NoErrorf(t, err, "mk%s %s", inoType, fullPath)
	}
	if setOwnerMode != nil {
		// <?uid>:<?gid>:<?mode>
		fields := strings.Split(*setOwnerMode, ":")
		require.Lenf(t, fields, 3, "set owner-mode format uid:gid:mode")
		uidStr, gidStr, modeStr := fields[0], fields[1], fields[2]

		if uidStr != "" && gidStr != "" {
			uid, err := strconv.Atoi(uidStr)
			require.NoErrorf(t, err, "chown %s: parse uid", fullPath)
			gid, err := strconv.Atoi(gidStr)
			require.NoErrorf(t, err, "chown %s: parse gid", fullPath)
			err = unix.Chown(fullPath, uid, gid)
			require.NoErrorf(t, err, "chown %s", fullPath)
		}

		if modeStr != "" {
			mode, err := strconv.ParseUint(modeStr, 8, 32)
			require.NoErrorf(t, err, "chmod %s: parse mode", fullPath)
			err = unix.Chmod(fullPath, uint32(mode))
			require.NoErrorf(t, err, "chmod %s", fullPath)
		}
	}
}

// CreateTree creates a rootfs tree using spec entries (as documented in
// [CreateInTree]). The returned path is the path to the root of the new tree.
func CreateTree(t TestingT, specs ...string) string {
	root := t.TempDir()

	// Put the root in a subdir.
	treeRoot := filepath.Join(root, "tree")
	MkdirAll(t, treeRoot, 0o755)

	for _, spec := range specs {
		CreateInTree(t, treeRoot, spec)
	}
	return treeRoot
}
