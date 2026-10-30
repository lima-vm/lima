#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Make the sshd_session_t SELinux domain permissive, so that sshd-session can forward
# guest Unix sockets (e.g., /run/lima-guestagent.sock, /run/podman/podman.sock) to the host.
#
# sshd_session_t used to be a permissive domain in Fedora, but selinux-policy 44.7 made it enforcing
# without allowing it to connect to such sockets.
# Commits:
# - https://github.com/fedora-selinux/selinux-policy/commit/d15be9cafa57892da09e5f37f8e48c7ff8496341 (Fedora 44, v44.7)
# - https://github.com/fedora-selinux/selinux-policy/commit/963c9ea0194da33f0058e8e6840ba484bb1c9aaa (Rawhide)
# https://github.com/lima-vm/lima/issues/5572
# https://github.com/fedora-selinux/selinux-policy/issues/3384

set -eu

command -v selinuxenabled >/dev/null 2>&1 || exit 0
selinuxenabled || exit 0
command -v semodule >/dev/null 2>&1 || exit 0

# Query the loaded policy via selinuxfs, instead of comparing the policy version.
# The query fails if the policy does not have sshd_session_t (older policies run sshd-session in sshd_t).
exec 3<>/sys/fs/selinux/access
ctx=system_u:system_r:sshd_session_t:s0
printf '%s %s %s' "${ctx}" "${ctx}" "$(cat /sys/fs/selinux/class/process/index)" >&3 2>/dev/null || exit 0
# allowed decided auditallow auditdeny seqno flags (hex), without a trailing newline
read -r _ _ _ _ _ flags <&3 || [ -n "${flags:-}" ]
exec 3>&-
# Nothing to do if sshd_session_t is already permissive (AVD_FLAGS_PERMISSIVE)
[ $((0x${flags} & 1)) = 0 ] || exit 0

module=lima-permissive-sshd-session
dir="$(mktemp -d)"
echo "(typepermissive sshd_session_t)" >"${dir}/${module}.cil"
echo "Installing SELinux module \"${module}\""
if ! semodule -i "${dir}/${module}.cil"; then
	echo >&2 "Failed to install SELinux module \"${module}\""
fi
rm -rf "${dir}"
