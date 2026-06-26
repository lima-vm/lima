#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux

# Pre-create /run/host-services owned by the guest user.
# This allows the hostagent to create the SSH agent socket symlink
# without requiring passwordless sudo.
mkdir -p /run/host-services
chmod 700 /run/host-services
chown -R "${LIMA_CIDATA_USER}" /run/host-services
