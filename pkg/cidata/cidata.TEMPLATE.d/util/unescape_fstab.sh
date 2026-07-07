#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Read a string on stdin and write it to stdout with the octal escapes used in
# /etc/fstab and /proc/mounts decoded to their literal characters:
# "\040" -> space, "\011" -> tab, "\012" -> newline, "\134" -> backslash,
# "\043" -> "#".
#
# This decodes the full set of escapes the kernel emits in /proc/mounts and that
# mount(8) reads from /etc/fstab -- a superset of what util/escape_fstab.sh
# produces (only \040, \011 and \134). It round-trips that script's output
# (unescape(escape(x)) == x) but is not its exact inverse.
# https://github.com/torvalds/linux/blob/v6.6/fs/proc_namespace.c#L89

set -eu

: "${SELFTEST:=}"
if [ -n "${SELFTEST}" ]; then
	unset SELFTEST
	tab=$(printf '\t')
	check() {
		local desc=$1 input=$2 want=$3 got
		got=$(printf '%s\n' "${input}" | "$0")
		if [ "${got}" = "${want}" ]; then
			echo "ok: ${desc}"
		else
			echo "FAIL: ${desc}" >&2
			printf '  want: %q\n  got:  %q\n' "${want}" "${got}" >&2
			return 1
		fi
	}
	echo >&2 "=== Running tests ==="
	check "spaces are decoded" '/tmp/dir\040with\040spaces' "/tmp/dir with spaces"
	check "tab is decoded" '/a\011b' "/a${tab}b"
	check "newline is decoded" 'a\012b' "$(printf 'a\nb')"
	check "backslash is decoded" '/a\134b' '/a\b'
	check "hash is decoded" '/a\043b' "/a#b"
	check "a string without escapes is unchanged" "/mnt/nospace" "/mnt/nospace"
	check "mixed escapes are decoded" '/a\040b\134c' '/a b\c'
	echo >&2 "=== All tests passed ==="
	exit 0
fi

sed -e 's/\\040/ /g; s/\\011/\t/g; s/\\012/\n/g; s/\\134/\\/g; s/\\043/#/g'
