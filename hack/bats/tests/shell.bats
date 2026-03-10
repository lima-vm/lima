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

@test 'limactl shell without --instance requires instance name as first argument' {
    run_e -1 limactl shell --tty=false
    assert_stderr --partial "requires instance name as first argument"
}

@test 'limactl shell nonexistent instance' {
    run_e -1 limactl shell --tty=false nonexistent
    assert_stderr --partial 'does not exist'
}

@test 'limactl shell --instance nonexistent instance' {
    run_e -1 limactl shell --tty=false --instance nonexistent
    assert_stderr --partial 'does not exist'
}

@test 'limactl shell --instance resolves instance name' {
    # --instance should resolve the instance name the same way as the positional arg
    run_e -1 limactl shell --tty=false --instance nonexistent
    inst_output=$stderr

    run_e -1 limactl shell --tty=false nonexistent
    assert_stderr "$inst_output"
}

@test 'limactl shell --instance with unknown flag errors' {
    # Without --, an unknown flag after --instance should be rejected by cobra
    run_e -1 limactl shell --tty=false --instance "$INSTANCE" --nonexistent-flag
    assert_stderr --partial "unknown flag"
}

@test 'limactl shell --instance with double dash stops flag parsing' {
    # With --, "--nonexistent-flag" must be treated as a command, not a flag.
    # The key assertion is that we do NOT get "unknown flag" (contrast with the test above).
    run_e limactl shell --tty=false --instance "$INSTANCE" -- --nonexistent-flag </dev/null
    refute_stderr --partial "unknown flag"
}

@test 'limactl shell positional instance with double dash' {
    # Same double-dash behavior with positional instance name
    run_e limactl shell --tty=false "$INSTANCE" -- --nonexistent-flag </dev/null
    refute_stderr --partial "unknown flag"
}

@test 'lima wrapper forwards unknown limactl shell flags' {
    # The lima wrapper should forward flags to limactl shell.
    # --start=false is a valid limactl shell flag; if the wrapper didn't forward it,
    # it would be treated as a command name and fail with "unknown flag".
    run_e env LIMA_INSTANCE="$INSTANCE" lima --tty=false --start=false </dev/null
    refute_stderr --partial "unknown flag"
}

@test 'lima wrapper rejects unknown flags' {
    # Flags not recognized by limactl shell should cause an error
    run_e -1 env LIMA_INSTANCE="$INSTANCE" lima --tty=false --nonexistent-flag
    assert_stderr --partial "unknown flag"
}
