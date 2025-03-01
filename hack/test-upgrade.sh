#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"
cd "${scriptdir}/.."

if [ "$#" -ne 2 ]; then
	ERROR "Usage: $0 OLDVER NEWVER"
	exit 1
fi

OLDVER="$1"
NEWVER="$2"

PREFIX="/usr/local"
function install_lima() {
	ver="$1"
	git checkout "${ver}"
	make clean
	make
	if [ -w "${PREFIX}/bin" ] && [ -w "${PREFIX}/share" ]; then
		make install
	else
		sudo make install
	fi
}

function install_lima_binary() {
	ver="$1"
	tar="tar"
	if [ ! -w "${PREFIX}/bin" ] || [ ! -w "${PREFIX}/share" ]; then
		tar="sudo ${tar}"
	fi
	curl -fsSL "https://github.com/lima-vm/lima/releases/download/${ver}/lima-${ver:1}-$(uname -s)-$(uname -m).tar.gz" | ${tar} Cxzvm "${PREFIX}"
}

function uninstall_lima() {
	files="${PREFIX}/bin/lima ${PREFIX}/bin/limactl ${PREFIX}/share/lima ${PREFIX}/share/doc/lima"
	if [ -w "${PREFIX}/bin" ] && [ -w "${PREFIX}/share" ]; then
		# shellcheck disable=SC2086
		rm -rf $files
	else
		# shellcheck disable=SC2086
		sudo rm -rf $files
	fi
}

function show_lima_log() {
	tail -n 100 ~/.lima/"${LIMA_INSTANCE}"/*.log || true
	mkdir -p failure-logs
	cp -pf ~/.lima/"${LIMA_INSTANCE}"/*.log failure-logs/ || true
	limactl shell "${LIMA_INSTANCE}" sudo cat /var/log/cloud-init-output.log | tee failure-logs/cloud-init-output.log || true
}

INFO "Uninstalling lima"
uninstall_lima

INFO "Installing the old Lima ${OLDVER}"
install_lima_binary "${OLDVER}" || install_lima "${OLDVER}"

export LIMA_INSTANCE="test-upgrade"

INFO "Creating an instance \"${LIMA_INSTANCE}\" with the old Lima"
defer "show_lima_log;limactl delete -f \"${LIMA_INSTANCE}\""
limactl start --tty=false --name="${LIMA_INSTANCE}" template://ubuntu-lts || (
	show_lima_log
	exit 1
)
lima nerdctl info

image_name="lima-test-upgrade-containerd-${RANDOM}"
image_context="${HOME}/${image_name}"
INFO "Building containerd image \"${image_name}\" from \"${image_context}\""
defer "rm -rf \"${image_context}\""
mkdir -p "${image_context}"
cat <<EOF >"${image_context}"/Dockerfile
# Use GHCR to avoid hitting Docker Hub rate limit
FROM ghcr.io/containerd/alpine:3.14.0
CMD ["echo", "Built with Lima ${OLDVER}"]
EOF
lima nerdctl build -t "${image_name}" "${image_context}"
lima nerdctl run --rm "${image_name}"

INFO "Stopping the instance"
limactl stop "${LIMA_INSTANCE}"

INFO "=============================================================================="

INFO "Installing the new Lima ${NEWVER}"
install_lima "${NEWVER}"

INFO "Restarting the instance"
limactl start --tty=false "${LIMA_INSTANCE}" || show_lima_log
lima nerdctl info

INFO "Confirming that the host filesystem is still mounted"
"${scriptdir}"/test-mount-home.sh "${LIMA_INSTANCE}"

INFO "Confirming that the image \"${image_name}\" still exists"
lima nerdctl run --rm "${image_name}"
