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
	"io"
)

type forceGetProcRootLevel int

const (
	forceGetProcRootDefault             forceGetProcRootLevel = iota
	forceGetProcRootOpenTree                                  // force open_tree()
	forceGetProcRootOpenTreeAtRecursive                       // force open_tree(AT_RECURSIVE)
	forceGetProcRootUnsafe                                    // force open()
)

var testingForceGetProcRoot *forceGetProcRootLevel

func testingCheckClose(check bool, f io.Closer) bool {
	if check {
		if f != nil {
			_ = f.Close()
		}
		return true
	}
	return false
}

func testingForcePrivateProcRootOpenTree(f io.Closer) bool {
	return testingForceGetProcRoot != nil &&
		testingCheckClose(*testingForceGetProcRoot >= forceGetProcRootOpenTree, f)
}

func testingForcePrivateProcRootOpenTreeAtRecursive(f io.Closer) bool {
	return testingForceGetProcRoot != nil &&
		testingCheckClose(*testingForceGetProcRoot >= forceGetProcRootOpenTreeAtRecursive, f)
}

func testingForceGetProcRootUnsafe() bool {
	return testingForceGetProcRoot != nil &&
		*testingForceGetProcRoot >= forceGetProcRootUnsafe
}

type forceProcThreadSelfLevel int

const (
	forceProcThreadSelfDefault forceProcThreadSelfLevel = iota
	forceProcSelfTask
	forceProcSelf
)

var testingForceProcThreadSelf *forceProcThreadSelfLevel

func testingForceProcSelfTask() bool {
	return testingForceProcThreadSelf != nil &&
		*testingForceProcThreadSelf >= forceProcSelfTask
}

func testingForceProcSelf() bool {
	return testingForceProcThreadSelf != nil &&
		*testingForceProcThreadSelf >= forceProcSelf
}

func init() {
	hookForceGetProcRootUnsafe = testingForceGetProcRootUnsafe
	hookForcePrivateProcRootOpenTree = testingForcePrivateProcRootOpenTree
	hookForcePrivateProcRootOpenTreeAtRecursive = testingForcePrivateProcRootOpenTreeAtRecursive

	hookForceProcSelf = testingForceProcSelf
	hookForceProcSelfTask = testingForceProcSelfTask
}
