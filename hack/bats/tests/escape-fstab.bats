# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

# Unit tests for pkg/cidata/cidata.TEMPLATE.d/util/escape-fstab.sh, the fstab
# mount-point escaper used by boot.Linux/05-lima-mounts.sh. Regression coverage
# for lima-vm/lima#5136. No Lima instance is required: the helper is a pure
# stdin->stdout filter, so these tests do not set INSTANCE.

ESCAPE_FSTAB="$(absolute_path "$PATH_BATS_ROOT/../..")/pkg/cidata/cidata.TEMPLATE.d/util/escape-fstab.sh"
TAB=$'\t'

@test 'a space in a cloud-config virtiofs mount point is octal-escaped' {
    run -0 "$ESCAPE_FSTAB" <<<"tag${TAB}/tmp/dir with spaces${TAB}virtiofs${TAB}rw,nofail,comment=cloudconfig${TAB}0${TAB}0"
    assert_output "tag${TAB}/tmp/dir\\040with\\040spaces${TAB}virtiofs${TAB}rw,nofail,comment=cloudconfig${TAB}0${TAB}0"
}

@test 'an already-escaped mount point is unchanged (idempotent)' {
    run -0 "$ESCAPE_FSTAB" <<<"tag${TAB}/tmp/dir\\040with\\040spaces${TAB}virtiofs${TAB}rw,nofail,comment=cloudconfig${TAB}0${TAB}0"
    assert_output "tag${TAB}/tmp/dir\\040with\\040spaces${TAB}virtiofs${TAB}rw,nofail,comment=cloudconfig${TAB}0${TAB}0"
}

@test 'a mount point without whitespace is unchanged' {
    run -0 "$ESCAPE_FSTAB" <<<"tag${TAB}/mnt/nospace${TAB}virtiofs${TAB}rw,nofail,comment=cloudconfig${TAB}0${TAB}0"
    assert_output "tag${TAB}/mnt/nospace${TAB}virtiofs${TAB}rw,nofail,comment=cloudconfig${TAB}0${TAB}0"
}

@test 'a backslash is escaped before the space' {
    run -0 "$ESCAPE_FSTAB" <<<"tag${TAB}/a b\\c${TAB}virtiofs${TAB}rw,comment=cloudconfig${TAB}0${TAB}0"
    assert_output "tag${TAB}/a\\040b\\134c${TAB}virtiofs${TAB}rw,comment=cloudconfig${TAB}0${TAB}0"
}

@test 'an entry without comment=cloudconfig is unchanged' {
    run -0 "$ESCAPE_FSTAB" <<<"tag${TAB}/tmp/dir with spaces${TAB}virtiofs${TAB}rw${TAB}0${TAB}0"
    assert_output "tag${TAB}/tmp/dir with spaces${TAB}virtiofs${TAB}rw${TAB}0${TAB}0"
}

@test 'a non-virtiofs entry is unchanged' {
    run -0 "$ESCAPE_FSTAB" <<<"/dev/sda1${TAB}/data dir${TAB}ext4${TAB}defaults,comment=cloudconfig${TAB}0${TAB}0"
    assert_output "/dev/sda1${TAB}/data dir${TAB}ext4${TAB}defaults,comment=cloudconfig${TAB}0${TAB}0"
}
