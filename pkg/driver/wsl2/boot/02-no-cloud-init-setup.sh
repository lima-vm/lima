#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This script replaces the cloud-init functionality of creating a user and setting its SSH keys
# when cloud-init is not available
[ "$LIMA_CIDATA_NO_CLOUD_INIT" = "1" ] || exit 0

# create user
# shellcheck disable=SC2153
useradd -u "${LIMA_CIDATA_UID}" "${LIMA_CIDATA_USER}" -c "${LIMA_CIDATA_COMMENT}" -d "${LIMA_CIDATA_HOME}" -m -s "${LIMA_CIDATA_SHELL}"
LIMA_CIDATA_GID=$(id -g "${LIMA_CIDATA_USER}")
mkdir "${LIMA_CIDATA_HOME}"/.ssh/
chown "${LIMA_CIDATA_UID}:${LIMA_CIDATA_GID}" "${LIMA_CIDATA_HOME}"/.ssh/
chmod 700 "${LIMA_CIDATA_HOME}"/.ssh/
cp "${LIMA_CIDATA_MNT}"/ssh_authorized_keys "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys
chown "${LIMA_CIDATA_UID}:${LIMA_CIDATA_GID}" "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys
chmod 600 "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys

# add $LIMA_CIDATA_USER to sudoers
echo "${LIMA_CIDATA_USER} ALL=(ALL) NOPASSWD:ALL" | tee -a /etc/sudoers.d/99_lima_sudoers

# symlink CIDATA to the hardcoded path for requirement checks (TODO: make this not hardcoded)
[ "$LIMA_CIDATA_MNT" = "/mnt/lima-cidata" ] || ln -sfFn "${LIMA_CIDATA_MNT}" /mnt/lima-cidata
