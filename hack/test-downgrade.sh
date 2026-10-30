#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests that an instance created with the new Lima can be started with the old Lima,
# and then with the new Lima again.

set -eu -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"
# shellcheck source=test-upgrade.inc.sh
source "${scriptdir}/test-upgrade.inc.sh"
cd "${scriptdir}/.."

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
	ERROR "Usage: $0 OLDVER NEWVER [TEMPLATE]"
	exit 1
fi

OLDVER="$1"
NEWVER="$2"
TEMPLATE="${3:-template:default}"

export LIMA_INSTANCE="test-downgrade"
INSTANCE_DIR="${HOME}/.lima/${LIMA_INSTANCE}"

# Connects without the ControlMaster of the host agent, so that the host key is verified.
function ssh_without_multiplexing() {
	ssh -F "${INSTANCE_DIR}/ssh.config" -o ControlMaster=no -o ControlPath=none "lima-${LIMA_INSTANCE}" "$@"
}

INFO "Uninstalling lima"
uninstall_lima

INFO "Installing the new Lima ${NEWVER}"
install_lima "${NEWVER}"
# Kept for running the new limactl while the old Lima is installed
NEW_LIMACTL="$(pwd)/_output/bin/limactl"

INFO "Creating an instance \"${LIMA_INSTANCE}\" with the new Lima"
defer "show_lima_log;limactl delete -f \"${LIMA_INSTANCE}\""
# LIMACTL_CREATE_ARGS is a list of arguments, so it must not be quoted.
# shellcheck disable=SC2086
limactl start --tty=false --name="${LIMA_INSTANCE}" ${LIMACTL_CREATE_ARGS:-} "${TEMPLATE}" || (
	show_lima_log
	exit 1
)
limactl shell "${LIMA_INSTANCE}" true
INFO "Confirming that the host key is verified"
grep -q "StrictHostKeyChecking yes" "${INSTANCE_DIR}/ssh.config"
ssh_without_multiplexing true

INFO "Stopping the instance"
limactl stop "${LIMA_INSTANCE}"
if [ ! -s "${INSTANCE_DIR}/ssh.known_hosts" ]; then
	ERROR "ssh.known_hosts must keep the host key pinned on the first boot"
	exit 1
fi

INFO "=============================================================================="

INFO "Installing the old Lima ${OLDVER}"
uninstall_lima
# Building OLDVER from the source with install_lima would replace the new build in _output.
install_lima_binary "${OLDVER}"

INFO "Restarting the instance with the old Lima"
limactl start --tty=false "${LIMA_INSTANCE}" || (
	show_lima_log
	exit 1
)
limactl shell "${LIMA_INSTANCE}" true

INFO "Confirming that the new limactl can connect to the instance running with the old Lima"
# --reconnect replaces the ControlMaster of the old host agent, so that the SSH options of the new limactl are used for the connection.
# The new limactl verifies the host key pinned by the new Lima, so the guest must keep the key while running with the old Lima.
"${NEW_LIMACTL}" shell --reconnect "${LIMA_INSTANCE}" true

INFO "Stopping the instance"
limactl stop "${LIMA_INSTANCE}"

INFO "=============================================================================="

INFO "Reinstalling the new Lima ${NEWVER}"
uninstall_lima
make_install

INFO "Restarting the instance with the new Lima"
limactl start --tty=false "${LIMA_INSTANCE}" || (
	show_lima_log
	exit 1
)
limactl shell "${LIMA_INSTANCE}" true
INFO "Confirming that the host key is verified"
grep -q "StrictHostKeyChecking yes" "${INSTANCE_DIR}/ssh.config"
ssh_without_multiplexing true
