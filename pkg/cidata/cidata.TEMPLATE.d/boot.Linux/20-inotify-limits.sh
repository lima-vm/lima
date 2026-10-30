#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Apply the `inotify` limits from lima.yaml. These are runtime values, so they are set again on every boot.

set -eu

sysctl -w fs.inotify.max_user_watches="${LIMA_CIDATA_INOTIFY_MAX_USER_WATCHES}"
sysctl -w fs.inotify.max_user_instances="${LIMA_CIDATA_INOTIFY_MAX_USER_INSTANCES}"
