// SPDX-License-Identifier: MPL-2.0

// Copyright (C) 2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fd_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
)

func TestNopCloser(t *testing.T) {
	f, err := os.Open("/")
	require.NoError(t, err)
	require.NotNil(t, f, "open /")

	actualName := f.Name()
	actualFd := f.Fd()

	f2 := fd.NopCloser(f)
	require.NotNil(t, f, "wrap f2")

	assert.NoError(t, f2.Close(), "close no-op")       //nolint:testifylint // this is an isolated operation so we can continue despite an error
	assert.NoError(t, f2.Close(), "close no-op again") //nolint:testifylint // this is an isolated operation so we can continue despite an error

	assert.Equal(t, actualFd, f2.Fd(), "fd should still be valid (file not closed)")
	assert.Equal(t, actualName, f2.Name(), "fd should still be valid (file not closed)")

	require.NoError(t, f.Close(), "close underlying file")

	assert.NotEqual(t, actualFd, f2.Fd(), "fd should not be valid (file closed)")
}
