#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eux -o pipefail

# Install required packages
dnf install -y dnf-plugins-core dnf-plugin-versionlock llvm18-libs

# Install Vulkan and Mesa base packages
dnf install -y \
	mesa-vulkan-drivers \
	vulkan-loader-devel \
	vulkan-headers \
	vulkan-tools \
	vulkan-loader \
	glslc

# Enable COPR repo with patched Mesa for Venus support
dnf copr enable -y slp/mesa-krunkit fedora-40-aarch64

# Downgrade to patched Mesa version from COPR
dnf downgrade -y mesa-vulkan-drivers.aarch64 \
	--repo=copr:copr.fedorainfracloud.org:slp:mesa-krunkit

# Lock Mesa version to prevent automatic upgrades
dnf versionlock add mesa-vulkan-drivers

# Clean up
dnf clean all

echo "Krunkit GPU(Venus) setup complete. Verify Vulkan installation by running 'vulkaninfo --summary'."
