# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests for `networkOpts.hostLoopback: false` with vz.
#
# A socat listener on the host's 127.0.0.1 stands in for a loopback-only host
# service. With the option set to false, a guest connection to it via
# host.lima.internal must be refused at once (rather than reaching the listener
# or hanging), while DNS keeps working. The same instance is then switched to
# true to show that the listener is reachable, so the refusal is not caused by
# the test setup.
#
# The tests share one instance and must run in order.
#
# Host requirements: macOS and socat.

load "../helpers/load"

NAME=bats-vz-host-loopback
HOST_PORT=17080
MESSAGE=host-loopback-ok

local_setup_file() {
    [[ $(uname) == "Darwin" ]] || skip "requires macOS host"
    if ! command -v socat >/dev/null; then
        echo "'socat' is required on the host to run these tests" >&2
        return 1
    fi

    socat "TCP-LISTEN:$HOST_PORT,bind=127.0.0.1,reuseaddr,fork" SYSTEM:"echo $MESSAGE" 3>&- 4>&- &
    echo $! >"$BATS_FILE_TMPDIR/socat.pid"

    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
    local template=$BATS_FILE_TMPDIR/$NAME.yaml
    cat >"$template" <<'YAML'
base: template:default
vmType: vz
networkOpts:
  hostLoopback: false
YAML
    # LIMACTL_CREATE_ARGS is a list of arguments, so it must not be quoted.
    # shellcheck disable=SC2086
    limactl start --yes --name "$NAME" ${LIMACTL_CREATE_ARGS:-} "$template" 3>&- 4>&-
}

local_teardown_file() {
    if [[ -s $BATS_FILE_TMPDIR/socat.pid ]]; then
        kill "$(cat "$BATS_FILE_TMPDIR/socat.pid")" || :
    fi
    delete_instance "$NAME"
    limactl delete --force "$NAME-user-v2" || :
}

bats::on_failure() {
    if [[ -d ${LIMA_HOME}/${NAME} ]]; then
        tail -n 100 "${LIMA_HOME}/${NAME}"/*.log || :
    fi
}

# Connect from the guest to the host listener via host.lima.internal. The
# timeout turns a hang into exit status 124, which no test accepts.
connect_from_guest() {
    limactl shell "$NAME" -- timeout 5 bash -c "exec 3<>/dev/tcp/host.lima.internal/$HOST_PORT && cat <&3"
}

@test "connections to a host loopback service via host.lima.internal are refused" {
    run connect_from_guest
    assert_failure 1
    assert_output --partial "Connection refused"
    refute_output --partial "$MESSAGE"
}

@test "DNS keeps working" {
    run -0 limactl shell "$NAME" -- resolvectl query --synthesize=no --cache=no host.lima.internal
    assert_output --partial "192.168.5.2"
}

@test "the host loopback service is reachable after setting hostLoopback to true" {
    limactl stop "$NAME"
    limactl edit --tty=false --set '.networkOpts.hostLoopback = true' "$NAME"
    limactl start "$NAME" 3>&- 4>&-
    # limactl may log warnings to stderr, so only compare stdout.
    run_e -0 connect_from_guest
    assert_output "$MESSAGE"
}

@test "hostLoopback false is rejected with a user-v2 network" {
    run -1 limactl start --tty=false --name "$NAME-user-v2" --vm-type=vz --network=lima:user-v2 \
        --set '.networkOpts.hostLoopback = false' template:default 3>&- 4>&-
    # The backticks are part of the message.
    # shellcheck disable=SC2016
    assert_output --partial '`networkOpts.hostLoopback: false` is not supported with a `user-v2` network'
}
