#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Install qemu on GitHub Actions runner.
# Not expected to be used outside GitHub Actions.

set -eux

# apt-get update has to be run beforehand
apt-get install -y --no-install-recommends ovmf qemu-system-x86 qemu-utils
modprobe kvm
# `usermod -aG kvm ${SUDO_USER}` does not take an effect on GHA
chown "${SUDO_USER}" /dev/kvm
qemu-system-x86_64 --version
