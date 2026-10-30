# shellcheck shell=bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Functions shared by test-upgrade.sh and test-downgrade.sh.
# Requires common.inc.sh to be sourced, and LIMA_INSTANCE to be set before calling show_lima_log.

PREFIX="${PREFIX:-/usr/local}"
export PREFIX # for make install
function install_lima() {
	ver="$1"
	git checkout "${ver}"
	make clean
	make
	make_install
}

# Installs the binaries that have been already built in _output
function make_install() {
	# PREFIX is passed explicitly, as sudo does not preserve the environment.
	if [ -w "${PREFIX}/bin" ] && [ -w "${PREFIX}/share" ] && [ -w "${PREFIX}/libexec" ]; then
		make install PREFIX="${PREFIX}"
	else
		sudo make install PREFIX="${PREFIX}"
	fi
}

function install_lima_binary() {
	ver="$1"
	tar="tar"
	if [ ! -w "${PREFIX}/bin" ] || [ ! -w "${PREFIX}/share" ] || [ ! -w "${PREFIX}/libexec" ]; then
		tar="sudo ${tar}"
	fi
	curl -fsSL "https://github.com/lima-vm/lima/releases/download/${ver}/lima-${ver:1}-$(uname -s)-$(uname -m).tar.gz" | ${tar} Cxzvm "${PREFIX}"
}

function uninstall_lima() {
	files="${PREFIX}/bin/lima ${PREFIX}/bin/limactl ${PREFIX}/share/lima ${PREFIX}/share/doc/lima ${PREFIX}/libexec/lima"
	if [ -w "${PREFIX}/bin" ] && [ -w "${PREFIX}/share" ] && [ -w "${PREFIX}/libexec" ]; then
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
