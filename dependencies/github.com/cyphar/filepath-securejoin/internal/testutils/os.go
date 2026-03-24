// SPDX-License-Identifier: MPL-2.0

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package testutils

import (
	"os"

	"github.com/stretchr/testify/require"
)

// Symlink is a wrapper around os.Symlink.
func Symlink(t TestingT, oldname, newname string) {
	err := os.Symlink(oldname, newname)
	require.NoError(t, err)
}

// MkdirAll is a wrapper around os.MkdirAll.
func MkdirAll(t TestingT, path string, mode os.FileMode) { //nolint:unparam // wrapper func
	err := os.MkdirAll(path, mode)
	require.NoError(t, err)
}

// WriteFile is a wrapper around os.WriteFile.
func WriteFile(t TestingT, path string, data []byte, mode os.FileMode) {
	err := os.WriteFile(path, data, mode)
	require.NoError(t, err)
}
