#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

# Define host.lima.internal in case the hostResolver is disabled. When using
# the hostResolver, the name is provided by the lima resolver itself because
# it doesn't have access to /etc/hosts inside the VM.
sed -i '/host.lima.internal/d' /etc/hosts
echo -e "${LIMA_CIDATA_SLIRP_GATEWAY}\thost.lima.internal" >>/etc/hosts
