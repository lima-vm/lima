#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

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

read -r -p "$(printf '\033[32mDo you want to download and build llama.cpp (for Vulkan) and install required packages? This may take a while. Proceed? [y/N]: \033[0m')" REPLY
case "$REPLY" in
[yY][eE][sS] | [yY]) ;;
*)
	echo "Aborted."
	exit 0
	;;
esac

echo "Installing llama.cpp with Vulkan support..."
# Build and install llama.cpp with Vulkan support
dnf install -y git cmake clang curl-devel glslc vulkan-devel virglrenderer
cd ~ && git clone https://github.com/ggml-org/llama.cpp && cd llama.cpp
git reset --hard 97340b4c9924be86704dbf155e97c8319849ee19
cmake -B build -DGGML_VULKAN=ON -DGGML_CCACHE=OFF -DCMAKE_INSTALL_PREFIX=/usr
cmake --build build --config Release -j8
cmake --install build
cd .. && rm -fr llama.cpp

echo "Successfully installed llama.cpp with Vulkan support. Use 'llama-cli' app with .gguf models."
