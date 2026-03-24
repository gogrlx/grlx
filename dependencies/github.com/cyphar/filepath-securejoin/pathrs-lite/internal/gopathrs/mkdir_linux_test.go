// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package gopathrs_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/gopathrs"
)

func TestMkdirAllHandle_InvalidMode(t *testing.T) { //nolint:revive // underscores are more readable for test helpers
	for _, test := range []struct {
		mode        os.FileMode
		expectedErr error
	}{
		// unix.S_IS* bits are invalid.
		{unix.S_ISUID | 0o777, gopathrs.ErrInvalidMode},
		{unix.S_ISGID | 0o777, gopathrs.ErrInvalidMode},
		{unix.S_ISVTX | 0o777, gopathrs.ErrInvalidMode},
		{unix.S_ISUID | unix.S_ISGID | unix.S_ISVTX | 0o777, gopathrs.ErrInvalidMode},
		// unix.S_IFMT bits are also invalid.
		{unix.S_IFDIR | 0o777, gopathrs.ErrInvalidMode},
		{unix.S_IFREG | 0o777, gopathrs.ErrInvalidMode},
		{unix.S_IFIFO | 0o777, gopathrs.ErrInvalidMode},
		// os.FileType bits are also invalid.
		{os.ModeDir | 0o777, gopathrs.ErrInvalidMode},
		{os.ModeNamedPipe | 0o777, gopathrs.ErrInvalidMode},
		{os.ModeIrregular | 0o777, gopathrs.ErrInvalidMode},
		// suid/sgid bits are silently ignored by mkdirat and so we return an
		// error explicitly.
		{os.ModeSetuid | 0o777, gopathrs.ErrInvalidMode},
		{os.ModeSetgid | 0o777, gopathrs.ErrInvalidMode},
		{os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0o777, gopathrs.ErrInvalidMode},
		// Proper sticky bit should work.
		{os.ModeSticky | 0o777, nil},
		// Regular mode bits.
		{0o777, nil},
		{0o711, nil},
	} {
		test := test // copy iterator
		t.Run(fmt.Sprintf("%s.%.3o", test.mode, test.mode), func(t *testing.T) {
			root := t.TempDir()
			rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
			require.NoError(t, err, "open root")
			defer rootDir.Close() //nolint:errcheck // test code

			handle, err := gopathrs.MkdirAllHandle(rootDir, "a/b/c", test.mode)
			require.ErrorIsf(t, err, test.expectedErr, "mkdirall %.3o (%s)", test.mode, test.mode)
			if test.expectedErr == nil {
				assert.NotNil(t, handle, "returned handle should be non-nil")
				_ = handle.Close()
			} else {
				assert.Nil(t, handle, "returned handle should be nil")
			}
		})
	}
}
