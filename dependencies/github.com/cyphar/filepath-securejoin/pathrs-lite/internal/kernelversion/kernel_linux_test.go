// SPDX-License-Identifier: BSD-3-Clause

// Copyright (C) 2025 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.BSD file.

package kernelversion

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKernelVersion(t *testing.T) {
	version, err := getKernelVersion()
	require.NoError(t, err)
	assert.GreaterOrEqualf(t, len(version), 2, "KernelVersion %#v must have at least 2 elements", version)
}

func TestParseKernelVersion(t *testing.T) {
	for _, test := range []struct {
		kverStr     string
		expected    KernelVersion
		expectedErr error
	}{
		// <2 components
		{"", nil, errInvalidKernelVersion},
		{"dummy", nil, errInvalidKernelVersion},
		{"1", nil, errInvalidKernelVersion},
		{"420", nil, errInvalidKernelVersion},
		// >=2 components
		{"3.7", KernelVersion{3, 7}, nil},
		{"3.8", KernelVersion{3, 8}, nil},
		{"3.8.0", KernelVersion{3, 8, 0}, nil},
		{"3.8.12", KernelVersion{3, 8, 12}, nil},
		{"3.8.12.10.0.2", KernelVersion{3, 8, 12, 10, 0, 2}, nil},
		{"42.12.1000", KernelVersion{42, 12, 1000}, nil},
		// suffix
		{"2.6.16foobar", KernelVersion{2, 6, 16}, nil},
		{"2.6.16f00b4r", KernelVersion{2, 6, 16}, nil},
		{"3.8.16-generic", KernelVersion{3, 8, 16}, nil},
		{"6.12.0-1-default", KernelVersion{6, 12, 0}, nil},
		{"4.9.27-default-foo.12.23", KernelVersion{4, 9, 27}, nil},
		// invalid version section
		{"-1.2", nil, errInvalidKernelVersion},
		{"3a", nil, errInvalidKernelVersion},
		{"3.a", nil, errInvalidKernelVersion},
		{"3.4.a", nil, errInvalidKernelVersion},
		{"a", nil, errInvalidKernelVersion},
		{"aa", nil, errInvalidKernelVersion},
		{"a.a", nil, errInvalidKernelVersion},
		{"a.a.a", nil, errInvalidKernelVersion},
		{"-3.1", nil, errInvalidKernelVersion},
		{"-3.", nil, errInvalidKernelVersion},
		{"1.-foo", nil, errInvalidKernelVersion},
		{".1", nil, errInvalidKernelVersion},
		{".1.2", nil, errInvalidKernelVersion},
	} {
		test := test // copy iterator
		t.Run(test.kverStr, func(t *testing.T) {
			kver, err := parseKernelVersion(test.kverStr)
			if test.expectedErr != nil {
				require.Errorf(t, err, "parseKernelVersion(%q)", test.kverStr)
				require.ErrorIsf(t, err, test.expectedErr, "parseKernelVersion(%q)", test.kverStr)
				assert.Nilf(t, kver, "parseKernelVersion(%q) returned kver", test.kverStr)
			} else {
				require.NoErrorf(t, err, "parseKernelVersion(%q)", test.kverStr)
				assert.Equal(t, test.expected, kver, "parseKernelVersion(%q) return kver", test.kverStr)
			}
		})
	}
}

func TestGreaterEqualThan(t *testing.T) {
	hostKver, err := getKernelVersion()
	require.NoError(t, err)

	for _, test := range []struct {
		name     string
		wantKver KernelVersion
		expected bool
	}{
		{"HostVersion", hostKver[:], true},
		{"OlderMajor", KernelVersion{hostKver[0] - 1, hostKver[1]}, true},
		{"OlderMinor", KernelVersion{hostKver[0], hostKver[1] - 1}, true},
		{"NewerMajor", KernelVersion{hostKver[0] + 1, hostKver[1]}, false},
		{"NewerMinor", KernelVersion{hostKver[0], hostKver[1] + 1}, false},
		{"ExtraDot", append(hostKver, 1), false},
		{"ExtraZeros", append(hostKver, make(KernelVersion, 10)...), true},
	} {
		test := test // copy iterator
		t.Run(fmt.Sprintf("%s:%s", test.name, test.wantKver), func(t *testing.T) {
			got, err := GreaterEqualThan(test.wantKver)
			require.NoErrorf(t, err, "GreaterEqualThan(%s)", test.wantKver)
			assert.Equalf(t, test.expected, got, "GreaterEqualThan(%s)", test.wantKver)
		})
	}
}
