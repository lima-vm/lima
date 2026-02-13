# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=bats

# TODO The reusable Lima instance setup is copied from preserve-env.bats
# TODO and should be factored out into helper functions.
local_setup_file() {
    if [[ -n "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        run limactl list --format '{{.Status}}' "$NAME"
        [[ $status == 0 ]] && [[ $output == "Running" ]] && return
    fi
    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
    # Make sure that the host agent doesn't inherit file handles 3 or 4.
    # Otherwise bats will not finish until the host agent exits.
    limactl start --yes --name "$NAME" template:default 3>&- 4>&-
}

local_teardown_file() {
    if [[ -z "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        limactl delete --force "$NAME"
    fi
}

@test "The guest home is accessible via both .guest and .linux paths" {
    run limactl shell "$NAME" -- ls -ld /home/"${USER}.guest/.ssh"
    assert_success

    run limactl shell "$NAME" -- ls -ld /home/"${USER}.linux/.ssh"
    assert_success
}