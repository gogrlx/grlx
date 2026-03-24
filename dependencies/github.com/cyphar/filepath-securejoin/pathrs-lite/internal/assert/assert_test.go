// SPDX-License-Identifier: MPL-2.0

// Copyright (C) 2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package assert_test

import (
	"errors"
	"testing"

	testassert "github.com/stretchr/testify/assert"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/assert"
)

func TestAssertTrue(t *testing.T) {
	for _, test := range []struct {
		name string
		val  any
	}{
		{"StringVal", "foobar"},
		{"IntVal", 123},
		{"ErrorVal", errors.New("error")},
		{"StructVal", struct{ a int }{1}},
		{"NilVal", nil},
	} {
		test := test // copy iterator
		t.Run(test.name, func(t *testing.T) {
			testassert.NotPanicsf(t, func() {
				assert.Assert(true, test.val)
			}, "assert(true) with value %v (%T)", test.val, test.val)
		})
	}

	t.Run("Assertf", func(t *testing.T) {
		fmtMsg := "foo %s %d"
		args := []any{"bar %x", 123}
		expected := "foo bar %x 123"

		testassert.NotPanicsf(t, func() {
			assert.Assertf(true, fmtMsg, args...)
		}, "assertf(true) with (%q, %v...) == %q", fmtMsg, args, expected)
	})
}

func TestAssertFalse(t *testing.T) {
	for _, test := range []struct {
		name string
		val  any
	}{
		{"StringVal", "foobar"},
		{"IntVal", 123},
		{"ErrorVal", errors.New("error")},
		{"StructVal", struct{ a int }{1}},
	} {
		test := test // copy iterator
		t.Run(test.name, func(t *testing.T) {
			testassert.PanicsWithValuef(t, test.val, func() {
				assert.Assert(false, test.val)
			}, "assert(false) with value %v (%T)", test.val, test.val)
		})
	}

	t.Run("NilVal", func(t *testing.T) {
		// testify can detect nil-value panics, but the behaviour of nil panics
		// changed in Go 1.21 (and can be modified by GODEBUG=panicnil=1) so we
		// can't be sure what value we will get.
		testassert.Panics(t, func() {
			assert.Assert(false, nil)
		}, "assert(false) with nil")
	})

	t.Run("Assertf", func(t *testing.T) {
		fmtMsg := "foo %s %d"
		args := []any{"bar %x", 123}
		expected := "foo bar %x 123"

		testassert.PanicsWithValuef(t, expected, func() {
			assert.Assertf(false, fmtMsg, args...)
		}, "assertf(true) with (%q, %v...) == %q", fmtMsg, args, expected)
	})
}
