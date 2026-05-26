#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

# The default guest home directory is "/home/${LIMA_CIDATA_USER}.guest" since Lima 2.1.0.
# The path was previously "/home/${LIMA_CIDATA_USER}.linux".
if [ -d "/home/${LIMA_CIDATA_USER}.guest" ] && [ ! -e "/home/${LIMA_CIDATA_USER}.linux" ]; then
	ln -s "${LIMA_CIDATA_USER}.guest" "/home/${LIMA_CIDATA_USER}.linux"
fi
if [ -d "/home/${LIMA_CIDATA_USER}.linux" ] && [ ! -e "/home/${LIMA_CIDATA_USER}.guest" ]; then
	ln -s "${LIMA_CIDATA_USER}.linux" "/home/${LIMA_CIDATA_USER}.guest"
fi
