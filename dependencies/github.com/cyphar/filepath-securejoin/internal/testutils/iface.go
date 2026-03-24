// SPDX-License-Identifier: MPL-2.0

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package testutils provides some internal helpers for tests.
package testutils

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestingT is an interface wrapper around *testing.T.
type TestingT interface {
	assert.TestingT
	require.TestingT

	TempDir() string
	Fatalf(format string, args ...any)
	Skip(args ...any)
}
