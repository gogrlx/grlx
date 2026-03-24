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
	"testing"
)

func testForceGetProcRoot(t *testing.T, testFn func(t *testing.T, expectOvermounts bool)) {
	for _, test := range []struct {
		name             string
		forceGetProcRoot forceGetProcRootLevel
		expectOvermounts bool
	}{
		{`procfd="fsopen()"`, forceGetProcRootDefault, false},
		{`procfd="open_tree_clone"`, forceGetProcRootOpenTree, false},
		{`procfd="open_tree_clone(AT_RECURSIVE)"`, forceGetProcRootOpenTreeAtRecursive, true},
		{`procfd="open()"`, forceGetProcRootUnsafe, true},
	} {
		test := test // copy iterator
		t.Run(test.name, func(t *testing.T) {
			testingForceGetProcRoot = &test.forceGetProcRoot
			defer func() { testingForceGetProcRoot = nil }()

			testFn(t, test.expectOvermounts)
		})
	}
}

func testForceProcThreadSelf(t *testing.T, testFn func(t *testing.T)) {
	for _, test := range []struct {
		name                string
		forceProcThreadSelf forceProcThreadSelfLevel
	}{
		{`thread-self="thread-self"`, forceProcThreadSelfDefault},
		{`thread-self="self/task"`, forceProcSelfTask},
		{`thread-self="self"`, forceProcSelf},
	} {
		test := test // copy iterator
		t.Run(test.name, func(t *testing.T) {
			testingForceProcThreadSelf = &test.forceProcThreadSelf
			defer func() { testingForceProcThreadSelf = nil }()

			testFn(t)
		})
	}
}
