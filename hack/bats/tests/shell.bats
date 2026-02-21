# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

INSTANCE=bats-dummy

@test 'lima stopped lima instance' {
    # check that the "tty" flag is used, also for stdin
    limactl shell --tty=false "$INSTANCE" true </dev/null
}

@test 'yes | stopped lima instance' {
    # check that stdin is verified and not just crashing
    bash -c "yes | limactl shell --tty=true $INSTANCE true"
}
