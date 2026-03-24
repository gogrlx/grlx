// SPDX-License-Identifier: BSD-3-Clause

// Copyright (C) 2017-2025 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securejoin

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Windows has very specific behaviour relating to volumes, and we can only
// test it on Windows machines because filepath.* behaviour depends on GOOS.
//
// See <https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats>
// for more information about the various path formats we need to make sure are
// correctly handled.
func TestHasDotDot_WindowsVolumes(t *testing.T) {
	for _, test := range []struct {
		testName, path string
		expected       bool
	}{
		{"plain-dotdot", `C:..`, true},            // apparently legal
		{"relative-dotdot", `C:..\foo\bar`, true}, // apparently legal
		{"trailing-dotdot", `D:\foo\bar\..`, true},
		{"leading-dotdot", `F:\..\foo\bar`, true},
		{"middle-dotdot", `F:\foo\..\bar`, true},
		{"drive-like-path", `\foo\C:..\bar`, false}, // C:.. is a filename here
		{"unc-dotdot", `\\gondor\share\call\for\aid\..\help`, true},
		{"dos-dotpath-dotdot1", `\\.\C:\..\foo\bar`, true},
		{"dos-dotpath-dotdot2", `\\.\C:\foo\..\bar`, true},
		{"dos-questionpath-dotdot1", `\\?\C:\..\foo\bar`, true},
		{"dos-questionpath-dotdot2", `\\?\C:\foo\..\bar`, true},
	} {
		test := test // copy iterator
		t.Run(test.testName, func(t *testing.T) {
			got := hasDotDot(test.path)
			assert.Equalf(t, test.expected, got, "unexpected result for hasDotDot(`%s`) (VolumeName: %q)", test.path, filepath.VolumeName(test.path))
		})
	}
}
