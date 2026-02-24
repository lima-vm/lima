#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

u="${LIMA_CIDATA_USER:-$USER}"
getent group render >/dev/null 2>&1 || groupadd -f render
getent group video >/dev/null 2>&1 || groupadd -f video
sudo usermod -aG render "$u" || true
sudo usermod -aG video "$u" || true
