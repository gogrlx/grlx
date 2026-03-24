// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestErrorXdev(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{"ErrPossibleAttack", ErrPossibleAttack},
		{"ErrPossibleBreakout", ErrPossibleBreakout},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.ErrorIs(t, test.err, test.err, "errors.Is(err, err) should succeed")     //nolint:useless-assert,testifylint // we need to check this
			assert.ErrorIs(t, test.err, unix.EXDEV, "errors.Is(err, EXDEV) should succeed") //nolint:useless-assert,testifylint // we need to check this
		})

		t.Run(test.name+"-Wrapped", func(t *testing.T) {
			err := fmt.Errorf("wrapped error: %w", test.err)
			assert.ErrorIs(t, err, test.err, "errors.Is(err, err) should succeed")     //nolint:useless-assert,testifylint // we need to check this
			assert.ErrorIs(t, err, unix.EXDEV, "errors.Is(err, EXDEV) should succeed") //nolint:useless-assert,testifylint // we need to check this
		})
	}
}
