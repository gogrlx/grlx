// SPDX-License-Identifier: BSD-3-Clause

//go:build linux

// Copyright (C) 2024 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gocompat

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoCompatErrorWrap(t *testing.T) {
	baseErr := errors.New("base error")
	extraErr := errors.New("extra error")

	err := WrapBaseError(baseErr, extraErr)

	require.Error(t, err)
	assert.ErrorIs(t, err, baseErr, "wrapped error should contain base error")   //nolint:testifylint // we are testing error behaviour directly
	assert.ErrorIs(t, err, extraErr, "wrapped error should contain extra error") //nolint:testifylint // we are testing error behaviour directly
}
