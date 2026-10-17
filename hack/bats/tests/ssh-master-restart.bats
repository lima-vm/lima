# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests that the hostagent restarts the SSH ControlMaster after it exits,
# restores the guestSocket forward and the SSH port forwarder's dynamic TCP
# forwards that the master held, and forwards a listener opened after the restart.
#
# Host requirements: socat.

load "../helpers/load"

NAME=bats-ssh-master-restart

GUEST_SOCKET=/tmp/ssh-master-restart.sock
TCP_PORT=34711       # forwarded by the default rule to the same host port
TCP_PORT_AFTER=34712 # opened only after the master restarts
RESTART_TIMEOUT=60   # seconds to wait for the master and the forwards to come back

local_setup_file() {
    case $OSTYPE in
    cygwin | msys) skip "the hostagent does not supervise the ControlMaster on Windows hosts" ;;
    esac
    if ! command -v socat >/dev/null; then
        echo "'socat' is required on the host to run these tests" >&2
        return 1
    fi

    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
    local template=$BATS_FILE_TMPDIR/$NAME.yaml
    cat >"$template" <<YAML
base: template:default
portForwards:
- guestSocket: $GUEST_SOCKET
  hostSocket: ssh-master-restart.sock
YAML
    # The SSH port forwarder puts the dynamic TCP forwards on the master.
    # LIMACTL_CREATE_ARGS is a list of arguments, so it must not be quoted.
    # shellcheck disable=SC2086
    LIMA_SSH_PORT_FORWARDER=true limactl start --yes --name "$NAME" ${LIMACTL_CREATE_ARGS:-} "$template" 3>&- 4>&-

    install_socat_in_guest "$NAME"
    start_listener_in_guest "UNIX-LISTEN:$GUEST_SOCKET,fork" unix-ok
    start_listener_in_guest "TCP4-LISTEN:$TCP_PORT,bind=127.0.0.1,fork,reuseaddr" tcp-ok
}

local_teardown_file() {
    delete_instance "$NAME"
}

# Start a socat server in the guest that answers every connection with REPLY.
start_listener_in_guest() {
    local address=$1 reply=$2
    limactl shell "$NAME" -- sh -c \
        "nohup socat $address SYSTEM:'echo $reply' >/dev/null 2>&1 </dev/null &"
}

host_socket() {
    echo "$(limactl list --format '{{.Dir}}' "$NAME")/sock/ssh-master-restart.sock"
}

# Print the pid of the running ControlMaster, or fail when none is running.
master_pid() {
    local ssh_config output
    ssh_config=$(limactl list --format '{{.SSHConfigFile}}' "$NAME")
    # `ssh -O check` prints "Master running (pid=N)" on stderr.
    output=$(ssh -F "$ssh_config" -O check "lima-$NAME" 2>&1) || return 1
    [[ $output =~ pid=([0-9]+) ]] || return 1
    echo "${BASH_REMATCH[1]}"
}

# Succeed when connecting to the socat address returns REPLY.
answers() {
    local address=$1 reply=$2
    [[ $(socat -T 5 -u "$address" STDOUT 2>/dev/null) == "$reply" ]]
}

# wait_until TIMEOUT CMD [ARGS...]: run CMD every second until it succeeds or TIMEOUT seconds pass.
wait_until() {
    local timeout=$1
    shift
    local deadline=$((SECONDS + timeout))
    until "$@"; do
        ((SECONDS < deadline)) || return 1
        sleep 1
    done
}

master_pid_differs_from() {
    local pid
    pid=$(master_pid) && [[ $pid != "$1" ]]
}

@test 'the forwards answer before the master restarts' {
    wait_until 30 answers "UNIX-CONNECT:$(host_socket)" unix-ok
    wait_until 30 answers "TCP4:127.0.0.1:$TCP_PORT" tcp-ok
}

@test 'the hostagent starts a new master after ssh -O exit' {
    local pid
    pid=$(master_pid)
    # --reconnect runs `ssh -O exit` on the master before running the command.
    limactl shell --reconnect "$NAME" true
    wait_until "$RESTART_TIMEOUT" master_pid_differs_from "$pid"
}

@test 'the forwards answer again after the master restarts' {
    wait_until "$RESTART_TIMEOUT" answers "UNIX-CONNECT:$(host_socket)" unix-ok
    wait_until "$RESTART_TIMEOUT" answers "TCP4:127.0.0.1:$TCP_PORT" tcp-ok
}

@test 'a listener opened after the master restarts is forwarded' {
    # Where the guest agent connects over SSH (QEMU), it reports new listeners
    # only once it has reconnected.
    start_listener_in_guest "TCP4-LISTEN:$TCP_PORT_AFTER,bind=127.0.0.1,fork,reuseaddr" tcp-after-ok
    wait_until "$RESTART_TIMEOUT" answers "TCP4:127.0.0.1:$TCP_PORT_AFTER" tcp-after-ok
}
