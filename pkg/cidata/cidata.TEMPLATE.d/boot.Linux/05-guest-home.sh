#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

# The default guest home directory is "/home/${LIMA_CIDATA_USER}.guest" since Lima 2.1.0.
# The path was previously "/home/${LIMA_CIDATA_USER}.linux".
#
# This must run after 04-persistent-data-volume.sh has mounted /home from the data
# volume, and after the WSL2 driver has created the user in 02-no-cloud-init-setup.sh.
# Symlinking earlier either targets a home directory that doesn't exist yet, or creates
# the symlink on the ramdisk, where the data volume immediately hides it.
if [ -d "/home/${LIMA_CIDATA_USER}.guest" ] && [ ! -e "/home/${LIMA_CIDATA_USER}.linux" ]; then
	ln -s "${LIMA_CIDATA_USER}.guest" "/home/${LIMA_CIDATA_USER}.linux"
fi
if [ -d "/home/${LIMA_CIDATA_USER}.linux" ] && [ ! -e "/home/${LIMA_CIDATA_USER}.guest" ]; then
	ln -s "${LIMA_CIDATA_USER}.linux" "/home/${LIMA_CIDATA_USER}.guest"
fi
