#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Integration test for runtime hot-mount (`limactl mount add|remove|list`).
# Requires the QEMU driver on a Linux host and an instance started by a Lima
# version that reserves the PCIe hot-plug slots (>= 2.1.0).

set -eu -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"

if [ "$#" -ne 1 ]; then
	ERROR "Usage: $0 NAME"
	exit 1
fi

NAME="$1"

# test_hot_mount TYPE
test_hot_mount() {
	local mtype="$1"
	local hosttmp guesttmp expected got
	hosttmp="$(mktemp -d "${TMPDIR:-/tmp}/lima-hot-mount.XXXXXX")"
	guesttmp="/mnt/lima-hot-mount-${mtype}"
	defer "rm -rf \"$hosttmp\""
	defer "limactl mount remove \"$NAME\" \"$guesttmp\" 2>/dev/null || true"

	expected="hot-mount-${mtype}-${RANDOM}"
	echo "$expected" >"$hosttmp/from-host"

	INFO "Hot-mounting ($mtype) \"$hosttmp\" on \"$guesttmp\""
	limactl mount add "$NAME" "$hosttmp" "$guesttmp" --type "$mtype" --writable

	INFO "Verifying it appears in 'mount list'"
	limactl mount list "$NAME" | grep -q "$guesttmp" || {
		ERROR "$guesttmp not listed"
		exit 1
	}

	INFO "Reading host-written file from the guest"
	got="$(limactl shell "$NAME" cat "$guesttmp/from-host")"
	if [ "$got" != "$expected" ]; then
		ERROR "host->guest read failed: expected=${expected}, got=${got}"
		exit 1
	fi

	INFO "Writing from the guest and reading back on the host"
	limactl shell "$NAME" sh -c "echo guest-wrote-${RANDOM} >\"$guesttmp/from-guest\""
	if [ ! -f "$hosttmp/from-guest" ]; then
		ERROR "guest->host write failed: $hosttmp/from-guest not found"
		exit 1
	fi

	INFO "Unmounting ($mtype)"
	limactl mount remove "$NAME" "$guesttmp"

	if limactl mount list "$NAME" | grep -q "$guesttmp"; then
		ERROR "$guesttmp still listed after remove"
		exit 1
	fi
	rm -rf "$hosttmp"
}

for mtype in virtiofs 9p reverse-sshfs; do
	INFO "=== hot-mount test: $mtype ==="
	test_hot_mount "$mtype"
done

INFO "Runtime hot-mount test passed"
