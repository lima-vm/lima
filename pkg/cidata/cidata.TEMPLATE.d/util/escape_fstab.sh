#!/bin/bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Read an /etc/fstab on stdin and write it to stdout with the mount-point field
# octal-escaped (per fstab(5)) for cloud-config virtiofs entries whose path
# contains a space or tab. cloud-init's cc_mounts writes the mount point verbatim,
# so a space/tab produces an unparsable line that mount(8) silently skips via the
# nofail option. Fields are tab-separated, so -F'\t' isolates the mount point;
# already-escaped paths have no literal space/tab, so the transformation is
# idempotent (and stays correct once cloud-init escapes the field itself).
#
# See:
# https://github.com/lima-vm/lima/issues/5136
# https://github.com/abiosoft/colima/issues/1471
# https://github.com/canonical/cloud-init/issues/3603 (cc_mounts does not escape)
# https://github.com/canonical/cloud-init/issues/6911 (the upstream cloud-init fix)

set -eu

: "${SELFTEST:=}"
if [ -n "${SELFTEST}" ]; then
	unset SELFTEST
	tab=$(printf '\t')
	check() {
		local desc=$1 input=$2 want=$3 got
		got=$(printf '%s\n' "${input}" | "$0")
		if [ "${got}" = "${want}" ]; then
			echo "ok: ${desc}"
		else
			echo "FAIL: ${desc}" >&2
			printf '  want: %q\n  got:  %q\n' "${want}" "${got}" >&2
			return 1
		fi
	}
	echo >&2 "=== Running tests ==="
	check "space in the mount point is escaped" \
		"tag${tab}/tmp/dir with spaces${tab}virtiofs${tab}rw,nofail,comment=cloudconfig${tab}0${tab}0" \
		"tag${tab}/tmp/dir\\040with\\040spaces${tab}virtiofs${tab}rw,nofail,comment=cloudconfig${tab}0${tab}0"
	check "already-escaped path is unchanged (idempotent)" \
		"tag${tab}/tmp/dir\\040with\\040spaces${tab}virtiofs${tab}rw,nofail,comment=cloudconfig${tab}0${tab}0" \
		"tag${tab}/tmp/dir\\040with\\040spaces${tab}virtiofs${tab}rw,nofail,comment=cloudconfig${tab}0${tab}0"
	check "path without whitespace is unchanged" \
		"tag${tab}/mnt/nospace${tab}virtiofs${tab}rw,nofail,comment=cloudconfig${tab}0${tab}0" \
		"tag${tab}/mnt/nospace${tab}virtiofs${tab}rw,nofail,comment=cloudconfig${tab}0${tab}0"
	check "backslash is escaped before the space" \
		"tag${tab}/a b\\c${tab}virtiofs${tab}rw,comment=cloudconfig${tab}0${tab}0" \
		"tag${tab}/a\\040b\\134c${tab}virtiofs${tab}rw,comment=cloudconfig${tab}0${tab}0"
	check "entry without comment=cloudconfig is unchanged" \
		"tag${tab}/tmp/dir with spaces${tab}virtiofs${tab}rw${tab}0${tab}0" \
		"tag${tab}/tmp/dir with spaces${tab}virtiofs${tab}rw${tab}0${tab}0"
	check "non-virtiofs entry is unchanged" \
		"/dev/sda1${tab}/data dir${tab}ext4${tab}defaults,comment=cloudconfig${tab}0${tab}0" \
		"/dev/sda1${tab}/data dir${tab}ext4${tab}defaults,comment=cloudconfig${tab}0${tab}0"
	echo >&2 "=== All tests passed ==="
	exit 0
fi

awk -F'\t' 'BEGIN { OFS = "\t" }
	{
		# The mount point can itself contain tabs (in the broken cloud-init case), so
		# don't assume fixed field positions. Find the first "virtiofs" field and
		# treat everything between $1 and that field as the mount point.
		fstype = 0
		for (i = 3; i <= NF; i++) {
			if ($i == "virtiofs") {
				fstype = i
				break
			}
		}
		if (fstype > 0 && (fstype + 3) <= NF) {
			src = $1
			mp = $2
			for (i = 3; i < fstype; i++) {
				mp = mp FS $i
			}
			opts = $(fstype + 1)
			dump = $(fstype + 2)
			pass = $(fstype + 3)
			if (opts ~ /comment=cloudconfig/ && mp ~ /[ \t]/) {
				p = mp
				gsub(/\\/, "\\134", p) # backslash first so introduced escapes are not re-escaped
				gsub(/ /, "\\040", p)
				gsub(/\t/, "\\011", p)
				mp = p
			}
			print src, mp, "virtiofs", opts, dump, pass
			next
		}
		print
	}'
