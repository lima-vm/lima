# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Build environment for Anthropic's OSS Scanner (https://github.com/anthropics/oss-scanner).
# The scanner builds this image online and then studies Lima inside it offline,
# so this file must fetch everything the build and the tests need.
FROM golang:1-trixie
ARG TARGETARCH

# limactl and its unit tests call ssh-keygen, qemu-img, rsync, visudo, bzip2, xz and zstd.
# QEMU, its UEFI firmware and its virtio-net ROM (ipxe-qemu) run the instance created below.
RUN case "${TARGETARCH}" in \
      amd64) qemu="qemu-system-x86 ovmf" ;; \
      arm64) qemu="qemu-system-arm qemu-efi-aarch64" ;; \
    esac \
    && apt-get update && apt-get install -y --no-install-recommends \
      bzip2 ipxe-qemu openssh-client qemu-utils rsync sudo xz-utils zstd ${qemu} \
    && rm -rf /var/lib/apt/lists/*

# limactl refuses to run as root, so give the scanner an unprivileged user to run it as.
RUN useradd --create-home lima

COPY . /src
WORKDIR /src

# DEBUG=1 keeps symbols and DWARF and disables optimization, for debuggers.
RUN make DEBUG=1 native && make install

# Keep the image even when tests fail, since the scanner can still use it.
RUN go test ./... || echo "WARNING: go test reported failures"

# Boot the default instance once while the network is up. The first boot downloads the image and
# the nerdctl archive and installs guest packages, so later boots work offline.
USER lima
RUN limactl start --tty=false --timeout=30m template:default && limactl stop default
