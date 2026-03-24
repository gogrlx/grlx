// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package procfs

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/linux"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/testutils"
)

func TestProcfsLookupInRoot(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// NOTE: We don't actually need root for unsafeHostProcRoot, but we
		// can't test for that because Go doesn't let you compare function
		// pointers...
		testutils.RequireRoot(t)

		// The openat2 and non-openat2 backends return different error
		// messages for the breakout case (".." and suspected magic-links).
		// The main issue is that openat2 just returns -EXDEV and returning
		// errUnsafeProcfs in all cases of the fallback resolver (for
		// consistency) doesn't make much sense.
		breakoutErr := internal.ErrPossibleBreakout
		if linux.HasOpenat2() {
			breakoutErr = errUnsafeProcfs
		}

		for _, test := range []struct {
			name          string
			root, subpath string
			expectedPath  string
			expectedErr   error
		}{
			{"nonproc-xdev", "/", "proc", "", errUnsafeProcfs},
			{"proc-nonroot", "/proc/tty", ".", "", errUnsafeProcfs},
			{"proc-emptypath", "/proc", "", "/proc", nil},
			{"proc-root-dotdot", "/proc", "1/../..", "", breakoutErr},
			{"proc-root-dotdot-top", "/proc", "..", "", breakoutErr},
			{"proc-abs-slash", "/proc", "/", "", breakoutErr},
			{"proc-abs-path", "/proc", "/etc/passwd", "", breakoutErr},
			// {"dotdot", "1/..", breakoutErr}, // only errors out for fallback resolver
			{"proc-uptime", "/proc", "uptime", "/proc/uptime", nil},
			{"proc-sys-kernel-arch", "/proc", "sys/kernel/arch", "/proc/sys/kernel/arch", nil},
			{"proc-symlink-nofollow", "/proc", "self", "/proc/self", nil},
			{"proc-symlink-follow", "/proc", "self/.", fmt.Sprintf("/proc/%d", os.Getpid()), nil},
			{"proc-self-attr", "/proc", "self/attr/apparmor/exec", fmt.Sprintf("/proc/%d/attr/apparmor/exec", os.Getpid()), nil},
			{"proc-magiclink-nofollow", "/proc", "self/exe", fmt.Sprintf("/proc/%d/exe", os.Getpid()), nil},
			{"proc-magiclink-follow", "/proc", "self/cwd/.", "", breakoutErr},
		} {
			test := test // copy iterator
			t.Run(test.name, func(t *testing.T) {
				root, err := os.Open(test.root)
				require.NoError(t, err, "open procfs resolver root")

				handle, err := procfsLookupInRoot(root, test.subpath)
				assert.ErrorIsf(t, err, test.expectedErr, "procfsLookupInRoot(%q)", test.subpath) //nolint:testifylint // this is an isolated operation so we can continue despite an error
				if handle != nil {
					handlePath, err := ProcSelfFdReadlink(handle)
					require.NoError(t, err, "ProcSelfFdReadlink handle")
					assert.Equal(t, test.expectedPath, handlePath, "ProcSelfFdReadlink of handle")
					_ = handle.Close()
				}
			})
		}
	})
}
