// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package procfs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/procfs"
)

// This code is all actually tested in internal/procfs, this is mainly
// necessary to make sure our one-line wrappers are correct.

func TestOpenProcRoot(t *testing.T) {
	t.Run("OpenProcRoot", func(t *testing.T) {
		proc, err := procfs.OpenProcRoot()
		require.NoError(t, err, "OpenProcRoot")
		assert.NotNil(t, proc, "procfs *Handle")
		assert.NoError(t, proc.Close(), "close handle")
	})

	t.Run("OpenUnsafeProcRoot", func(t *testing.T) {
		proc, err := procfs.OpenUnsafeProcRoot()
		require.NoError(t, err, "OpenUnsafeProcRoot")
		assert.NotNil(t, proc, "procfs *Handle")
		defer proc.Close() //nolint:errcheck // test code

		// Make sure the handle actually is !subset=pid.
		f, err := proc.OpenRoot(".")
		require.NoError(t, err, "open root .")
		err = fd.Faccessat(f, "uptime", unix.F_OK, unix.AT_SYMLINK_NOFOLLOW)
		assert.NoError(t, err, "/proc/uptime should exist") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		assert.NoError(t, proc.Close(), "close handle")
	})
}

type procRootFunc func() (*procfs.Handle, error)

func TestProcRoot(t *testing.T) {
	for _, test := range []struct {
		name       string
		procRootFn procRootFunc
	}{
		{"OpenProcRoot", procfs.OpenProcRoot},
		{"OpenUnsafeProcRoot", procfs.OpenUnsafeProcRoot},
	} {
		test := test // copy iterator
		t.Run(test.name, func(t *testing.T) {
			proc, err := test.procRootFn()
			require.NoError(t, err)
			defer proc.Close() //nolint:errcheck // test code

			t.Run("OpenThreadSelf", func(t *testing.T) {
				// Make sure our tid checks below are correct.
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()

				stat, closer, err := proc.OpenThreadSelf("stat")
				require.NoError(t, err, "open /proc/thread-self/stat")
				if assert.NotNil(t, closer, "closer should be non-nil for /proc/thread-self") {
					defer closer()
				}
				require.NotNil(t, stat, "open /proc/thread-self/stat")
				defer stat.Close() //nolint:errcheck // test code

				statData, err := os.ReadFile(fmt.Sprintf("/proc/self/fd/%d", stat.Fd()))
				runtime.KeepAlive(stat)
				require.NoError(t, err)
				assert.Regexp(t, fmt.Sprintf("^%d ", unix.Gettid()), string(statData), "/proc/thread-self/stat should have tid prefix")

				// Confirm that this is /proc/$pid/task/$tid, not /proc/$pid.
				f, closer, err := proc.OpenThreadSelf("task")
				require.ErrorIs(t, err, os.ErrNotExist, "/proc/thread-self should not have a 'task' dir")
				if !assert.Nil(t, closer, "returned closer on error") {
					defer closer()
				}
				if !assert.Nil(t, f, "returned *os.File on error") {
					_ = f.Close()
				}
			})

			t.Run("OpenSelf", func(t *testing.T) {
				stat, err := proc.OpenSelf("stat")
				require.NoError(t, err, "open /proc/self/stat")
				require.NotNil(t, stat, "open /proc/self/stat")
				defer stat.Close() //nolint:errcheck // test code

				statData, err := os.ReadFile(fmt.Sprintf("/proc/self/fd/%d", stat.Fd()))
				runtime.KeepAlive(stat)
				require.NoError(t, err)
				assert.Regexp(t, fmt.Sprintf("^%d ", os.Getpid()), string(statData), "/proc/self/stat should have pid prefix")

				// Confirm that this is /proc/$pid, not /proc/$pid/task/$tid.
				f, err := proc.OpenSelf("task")
				require.NoError(t, err, "/proc/self has a 'task' dir")
				require.NotNil(t, f, "open /proc/self/task")
				_ = f.Close()
			})

			t.Run("OpenPid", func(t *testing.T) {
				stat, err := proc.OpenPid(1, "stat")
				require.NoError(t, err, "open /proc/1/stat")
				require.NotNil(t, stat, "open /proc/1/stat")
				defer stat.Close() //nolint:errcheck // test code

				statData, err := os.ReadFile(fmt.Sprintf("/proc/self/fd/%d", stat.Fd()))
				runtime.KeepAlive(stat)
				require.NoError(t, err)
				assert.Regexp(t, "^1 ", string(statData), "/proc/1/stat should have pid1 prefix")

				// Confirm that this is /proc/$pid, not /proc/$pid/task/$tid.
				f, err := proc.OpenPid(1, "task")
				require.NoError(t, err, "/proc/1 has a 'task' dir")
				require.NotNil(t, f, "open /proc/1/task")
				_ = f.Close()
			})

			t.Run("OpenRoot", func(t *testing.T) {
				uptime, err := proc.OpenRoot("uptime")
				require.NoError(t, err, "open /proc/uptime")
				require.NotNil(t, uptime, "open /proc/uptime")
				defer uptime.Close() //nolint:errcheck // test code
			})
		})
	}
}

func TestProcSelfFdReadlink(t *testing.T) {
	root, err := os.Open(".")
	require.NoError(t, err)

	fullPath, err := procfs.ProcSelfFdReadlink(root)
	require.NoError(t, err, "ProcSelfFdReadlink")

	cwd, err := os.Getwd()
	require.NoError(t, err, "getwd")
	cwd, err = filepath.EvalSymlinks(cwd)
	require.NoError(t, err, "expand symlinks getwd")

	assert.Equal(t, cwd, fullPath, "ProcSelfFdReadlink('.')")
}
