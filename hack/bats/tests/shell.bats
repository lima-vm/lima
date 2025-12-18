# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=dummy

local_setup_file() {
    for INSTANCE in "$NAME"; do
        limactl delete --force "$INSTANCE" || :
    done
}

@test 'create dummy instance' {
    run -0 create_dummy_instance "$NAME" '.disk = "1M"'
}

@test 'lima stopped lima instance' {
    # check that the "tty" flag is used, also for stdin
    limactl shell --tty=false "$NAME" true </dev/null
}

@test 'yes | stopped lima instance' {
    # check that stdin is verified and not just crashing
    bash -c "yes | limactl shell --tty=true $NAME true"
}

@test 'delete dummy instance' {
    run_e -0 limactl delete --force "$NAME"
    assert_info "Deleted \"${NAME}\"…"
}
