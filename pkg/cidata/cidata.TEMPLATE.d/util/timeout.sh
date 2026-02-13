#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Reimplementation of `timeout` command for non-GNU operating systems,
# such as macOS.

set -eu -o pipefail

if [ "$#" -lt 2 ]; then
	echo "Usage: $0 DURATION COMMAND [ARG]..." >&2
	exit 1
fi

timeout() {
	local time=$1
	shift
	"$@" &
	local pid=$!
	(
		sleep "$time"
		kill -TERM "$pid" 2>/dev/null
	) &
	local killer=$!
	wait "$pid"
	local status=$?
	kill -TERM "$killer" 2>/dev/null
	return $status
}

timeout "$@"
