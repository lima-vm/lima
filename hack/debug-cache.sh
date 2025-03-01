#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail
cache_dir="${HOME}/Library/Caches"
if [ "$(uname -s)" != "Darwin" ]; then
	cache_dir="${HOME}/.cache"
fi
if [ ! -e "${cache_dir}/lima/download/by-url-sha256" ]; then
	echo "No cache"
	exit 0
fi
for f in "${cache_dir}/lima/download/by-url-sha256/"*; do
	echo "$f"
	ls -l "$f"
	cat "${f}/url"
	echo
	echo ---
done
