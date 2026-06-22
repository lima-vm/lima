#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Read an /etc/fstab on stdin and write it to stdout with the mount-point field
# octal-escaped (per fstab(5)) for cloud-config virtiofs entries whose path
# contains a space or tab.
#
# cloud-init's cc_mounts writes the mount point verbatim with a bare
# "\t".join(fields), so a space/tab in the path produces an unparsable line that
# mount(8) silently skips because of the nofail option
# (lima-vm/lima#5136, abiosoft/colima#1471, canonical/cloud-init#3603).
#
# Fields are tab-separated, so -F'\t' isolates the mount point even when it
# contains a space. Already-escaped paths contain no literal space/tab, so the
# transformation is idempotent (and stays correct once cloud-init escapes the
# field itself: canonical/cloud-init#6911).
#
# Unit tests: hack/bats/tests/escape-fstab.bats

set -eu

awk -F'\t' 'BEGIN { OFS = "\t" }
	$3 == "virtiofs" && $4 ~ /comment=cloudconfig/ && $2 ~ /[ \t]/ {
		p = $2
		gsub(/\\/, "\\134", p) # backslash first so introduced escapes are not re-escaped
		gsub(/ /, "\\040", p)
		gsub(/\t/, "\\011", p)
		$2 = p
	}
	{ print }'
