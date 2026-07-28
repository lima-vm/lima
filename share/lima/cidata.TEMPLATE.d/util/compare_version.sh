#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

: "${SELFTEST:=}"
if [ -n "$SELFTEST" ]; then
	unset SELFTEST
	echo >&2 "=== Running positive tests ==="
	(
		set -x
		"$0" 0.1.2 -eq 0.1.2
		"$0" 0.1.2 -ne 0.1.3
		"$0" 0.1.2 -ge 0.1.1
		"$0" 0.1.2 -ge 0.1.2
		"$0" 0.1.10 -ge 0.1.9
		"$0" 0.1.2 -gt 0.1.1
		"$0" 0.1.10 -gt 0.1.9
		"$0" 0.1.2 -le 0.1.2
		"$0" 0.1.2 -le 0.1.3
		"$0" 0.1.2 -le 0.1.10
		"$0" 0.1.2 -lt 0.1.3
		"$0" 0.1.2 -lt 0.1.10
	)
	echo >&2 "=== Running negative tests ==="
	(
		set -x
		"$0" 0.1.2 -eq 0.1.1 && false
		"$0" 0.1.2 -ne 0.1.2 && false
		"$0" 0.1.2 -ge 0.1.3 && false
		"$0" 0.1.2 -gt 0.1.2 && false
		"$0" 0.1.2 -le 0.1.1 && false
		"$0" 0.1.2 -lt 0.1.2 && false
		true
	)
	exit 0
fi

if [ "$#" -ne 3 ]; then
	echo >&2 "Usage: $0 VERSION-A OP VERSION-B"
	echo >&2 "Implemented operators: -eq, -ne, -ge, -gt, -le, -lt"
	echo >&2 ""
	echo >&2 "Example: $0 1.2.10 -ge 1.2.9"
	exit 1
fi

version_a="$1"
op="$2"
version_b="$3"

sorted="$(echo -ne "${version_a}\n${version_b}\n" | sort -V -r | head -n1)"
case "${op}" in
-eq)
	[ "${version_a}" = "${version_b}" ]
	;;
-ne)
	[ "${version_a}" != "${version_b}" ]
	;;
-ge)
	[ "${version_a}" = "${sorted}" ]
	;;
-gt)
	[ "${version_a}" = "${sorted}" ] && [ "${version_a}" != "${version_b}" ]
	;;
-le)
	[ "${version_b}" = "${sorted}" ]
	;;
-lt)
	[ "${version_b}" = "${sorted}" ] && [ "${version_a}" != "${version_b}" ]
	;;
*)
	echo "Unknown operator \"$op\""
	exit 1
	;;
esac
