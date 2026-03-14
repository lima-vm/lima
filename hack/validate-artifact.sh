#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0
#
# This script validates that lima-<VERSION>-Darwin-arm64.tar.gz
# contains lima-guestagent.Linux-aarch64
# but does not contain share/lima/lima-guestagent.Linux-x86_64

set -eu -o pipefail

must_contain() {
	tmp="$(mktemp)"
	tar tzf "$1" >"$tmp"
	if ! grep -q "$2" "$tmp"; then
		echo >&2 "ERROR: $1 must contain $2"
		cat "$tmp"
		rm -f "$tmp"
		exit 1
	fi
	rm -f "$tmp"
}

must_not_contain() {
	tmp="$(mktemp)"
	tar tzf "$1" >"$tmp"
	if grep -q "$2" "$tmp"; then
		echo >&2 "ERROR: $1 must not contain $2"
		cat "$tmp"
		rm -f "$tmp"
		exit 1
	fi
	rm -f "$tmp"
}

validate_artifact() {
	FILE="$1"
	MYARCH="x86_64"
	OTHERARCH="aarch64"
	if [[ $FILE == *"aarch64"* || $FILE == *"arm64"* ]]; then
		MYARCH="aarch64"
		OTHERARCH="x86_64"
	fi
	if [[ $FILE == *"go-mod-vendor.tar.gz" ]]; then
		: NOP
	elif [[ $FILE == *"lima-additional-guestagents"*".tar.gz" ]]; then
		must_not_contain "$FILE" "lima-guestagent.Linux-$MYARCH"
		must_contain "$FILE" "lima-guestagent.Linux-$OTHERARCH"
	elif [[ $FILE == *"lima-"*".tar.gz" ]]; then
		must_not_contain "$FILE" "lima-guestagent.Linux-$OTHERARCH"
		must_contain "$FILE" "lima-guestagent.Linux-$MYARCH"
	else
		echo >&2 "ERROR: Unexpected file: $FILE"
		exit 1
	fi
}

for FILE in "$@"; do
	validate_artifact "$FILE"
done
