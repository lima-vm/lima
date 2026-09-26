# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=bats-snapshot

local_setup_file() {
    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
    # LIMACTL_CREATE_ARGS is a list of arguments, so it must not be quoted.
    # shellcheck disable=SC2086
    limactl create --tty=false --name "$NAME" --mount-none \
        --set '.cpus=2' --set '.memory="2GiB"' \
        ${LIMACTL_CREATE_ARGS:-} template:default
}

local_teardown_file() {
    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
}

@test 'stopped snapshot restores a guest file' {
    run -0 limactl list --format '{{.VMType}}' "$NAME"
    case $output in
    qemu | krunkit) ;;
    *) skip "vmType ${output} does not implement snapshots" ;;
    esac

    limactl start --tty=false "$NAME"
    before=$(limactl shell "$NAME" -- sh -c 'echo marker-1 >/var/tmp/snap-marker && sync && sha256sum /var/tmp/snap-marker')
    limactl stop "$NAME"

    run -0 limactl snapshot create "$NAME" --tag base
    run -0 limactl snapshot list "$NAME" --quiet
    assert_line base

    limactl start --tty=false "$NAME"
    after=$(limactl shell "$NAME" -- sh -c 'echo marker-2 >/var/tmp/snap-marker && sync && sha256sum /var/tmp/snap-marker')
    [[ $after != "$before" ]]
    limactl stop "$NAME"

    run -0 limactl snapshot apply "$NAME" --tag base
    limactl start --tty=false "$NAME"
    run -0 limactl shell "$NAME" -- sha256sum /var/tmp/snap-marker
    assert_output "$before"

    limactl stop "$NAME"
    run -0 limactl snapshot delete "$NAME" --tag base
    run -0 limactl snapshot list "$NAME" --quiet
    refute_line base
}
