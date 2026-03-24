#!/bin/bash
# SPDX-License-Identifier: MPL-2.0

# Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
# Copyright (C) 2024-2025 SUSE LLC
#
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -Eeuo pipefail

root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"
pushd "$root"

GO="${GO:-go}"

silent=
verbose=
long=
libpathrs=
while getopts "svLl" opt; do
	case "$opt" in
		s)
			silent=1
			;;
		v)
			verbose=1
			;;
		L)
			long=1
			;;
		l)
			libpathrs=1
			;;
		*)
			echo "$0 [-s(ilent)]"
			exit 1
	esac
done

gocoverdir="$(mktemp --tmpdir -d gocoverdir.XXXXXXXX)"
trap 'rm -rf $gocoverdir' EXIT

test_args=("-count=1" "-cover" "-coverpkg=./...")
[ -n "$verbose" ] && test_args+=("-v")
[ -z "$long" ] && test_args+=("-short")
[ -n "$libpathrs" ] && test_args+=("-tags" "libpathrs")

"$GO" test "${test_args[@]}" ./... -args -test.gocoverdir="$gocoverdir"
sudo "$GO" test "${test_args[@]}" ./... -args -test.gocoverdir="$gocoverdir"

"$GO" tool covdata percent -i "$gocoverdir"
[ -n "$silent" ] || "$GO" tool covdata func -i "$gocoverdir" | sort -k 3gr

gocoverage="$(mktemp gocoverage.XXXXXXXX)"
trap 'rm $gocoverage' EXIT

"$GO" tool covdata textfmt -i "$gocoverdir" -o "$gocoverage"
[ -n "$silent" ] || "$GO" tool cover -html="$gocoverage"
