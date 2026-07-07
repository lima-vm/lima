# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests for the TCP port-forwarding rules of the hostagent.
#
# Every test follows the same pattern:
#
# 1. Start a socat listener on a specific address inside the guest.
# 2. Wait for the hostagent to emit a "forwarding" (or "not-forwarding") event
#    for that address. The events are recorded by a `limactl watch --json`
#    process started in local_setup_file.
# 3. Send a message from the host to the forwarded address. It must arrive at
#    the guest listener verbatim (or, for ignored listeners, must not arrive).
#
# The port forwarder is agnostic to the guest template, so this file only runs
# with the default template. Its behavior does depend on the host OS though,
# so it should be run on every supported host platform.
#
# Host requirements: socat and jq.

load "../helpers/load"

NAME=bats-port-forwarding

# The instance is created from this base template, with the portForwards rules
# below added. CI overrides it to test the WSL2 driver on Windows hosts.
BASE_TEMPLATE=${LIMA_BATS_PORT_FORWARDING_BASE_TEMPLATE:-template:default}

EVENT_TIMEOUT=30   # seconds to wait for a hostagent port-forwarding event
RECEIVE_TIMEOUT=10 # seconds to wait for a message to arrive in the guest

# The portForwards rules under test. Rule order matters: the first rule matching
# a guest listener address wins. The @test cases below are in the same order as
# the rules they exercise.
#
# HOST_LAN_IP is replaced with the actual LAN IP address of the host, to test
# forwarding to a host address other than 127.0.0.1.
#
# Untestable here: that the sshLocalPort itself is never forwarded (the
# guestagent ignores it from startup, before the event recorder is running),
# and connecting to a 0.0.0.0 forward from a non-loopback host interface.
PORT_FORWARDING_RULES="$(cat <<'YAML'
portForwards:
- guestIP: 127.0.0.2
  guestPortRange: [3000, 3009]
  hostPortRange: [2000, 2009]
  ignore: true

- guestIP: 0.0.0.0
  guestIPMustBeZero: false
  guestPortRange: [3010, 3019]
  hostPortRange: [2010, 2019]
  ignore: true

- guestIP: 0.0.0.0
  guestIPMustBeZero: false
  guestPortRange: [3000, 3029]
  hostPortRange: [2000, 2029]

# This rule is completely shadowed by the previous rule and has no effect.
- guestIP: 0.0.0.0
  guestIPMustBeZero: false
  guestPortRange: [3020, 3029]
  hostPortRange: [2020, 2029]
  ignore: true

- guestPortRange: [3030, 3039]
  hostPortRange: [2030, 2039]
  hostIP: HOST_LAN_IP

# The host ports of the following four rules are privileged (below 1024);
# the hostagent can bind them only on macOS.
- guestPortRange: [300, 304]

- guestPortRange: [305, 309]
  guestIPMustBeZero: false

- guestPortRange: [310, 314]
  hostIP: 0.0.0.0

- guestPortRange: [315, 319]
  guestIPMustBeZero: false
  hostIP: 0.0.0.0

# 192.168.5.15 is the guest address on the user-mode network.
- guestIP: "192.168.5.15"
  guestPortRange: [4000, 4009]
  hostIP: HOST_LAN_IP

- guestIP: "::1"
  guestPortRange: [4010, 4019]
  hostIP: "::"

- guestIP: "::"
  guestPortRange: [4020, 4029]
  hostIP: HOST_LAN_IP

- guestIP: "0.0.0.0"
  guestIPMustBeZero: false
  guestPortRange: [4030, 4039]
  hostIP: HOST_LAN_IP

- guestIPMustBeZero: true
  guestPortRange: [4040, 4049]

- guestIP: "0.0.0.0"
  guestIPMustBeZero: false
  guestPortRange: [4040, 4049]
  ignore: true

# Relative hostSocket paths are created inside the "sock" subdirectory of the
# instance directory.
- guestPort: 5000
  hostSocket: port5000.sock

- guestPort: 5001
  hostSocket: port5001.sock

- guestPort: 5002
  guestIPMustBeZero: false
  hostSocket: port5002.sock

- guestPort: 5003
  guestIPMustBeZero: false
  hostSocket: port5003.sock
YAML
)"

local_setup_file() {
    local cmd
    for cmd in socat jq; do
        if ! command -v "$cmd" >/dev/null; then
            echo "'$cmd' is required on the host to run these tests" >&2
            return 1
        fi
    done

    if ! reusing_running_instance; then
        limactl unprotect "$NAME" || :
        limactl delete --force "$NAME" || :
        local template=$BATS_FILE_TMPDIR/$NAME.yaml
        echo "base: $BASE_TEMPLATE" >"$template"
        echo "${PORT_FORWARDING_RULES//HOST_LAN_IP/$(host_lan_ip)}" >>"$template"
        # LIMACTL_CREATE_ARGS is a list of arguments, so it must not be quoted.
        # shellcheck disable=SC2086
        limactl start --yes --name "$NAME" ${LIMACTL_CREATE_ARGS:-} "$template" 3>&- 4>&-
    fi

    # The listeners for the tests run inside the guest, but the guest image may
    # not include socat.
    install_socat_in_guest
    # Kill listeners left over from a previous run (only relevant with LIMA_BATS_REUSE_INSTANCE).
    limactl shell "$NAME" -- sh -c 'sudo pkill -x socat; rm -f /tmp/port-forwarding.*.out; true'

    # Record hostagent events for wait_for_port_forwarding_event.
    limactl watch --json "$NAME" >"$BATS_FILE_TMPDIR/events" 2>"$BATS_FILE_TMPDIR/watch.log" 3>&- 4>&- &
    echo $! >"$BATS_FILE_TMPDIR/watch.pid"
}

local_teardown_file() {
    if [[ -s $BATS_FILE_TMPDIR/watch.pid ]]; then
        kill "$(cat "$BATS_FILE_TMPDIR/watch.pid")" || :
    fi
    delete_instance "$NAME"
}

# When a test fails, show the events received so far to aid debugging.
local_teardown() {
    if [[ -z ${BATS_TEST_COMPLETED:-} && -z ${BATS_TEST_SKIPPED:-} && -s $BATS_FILE_TMPDIR/events ]]; then
        echo "hostagent port-forwarding events received so far:"
        # "|| :" because the watch process is still writing, so the last line may
        # be a partial JSON object that makes jq fail under pipefail.
        jq -c '.event.status.portForward | select(. != null)' "$BATS_FILE_TMPDIR/events" | tail -n 25 || :
    fi
}

reusing_running_instance() {
    [[ -n ${LIMA_BATS_REUSE_INSTANCE:-} ]] || return 1
    run limactl list --format '{{.Status}}' "$NAME"
    [[ $status == 0 && $output == "Running" ]]
}

install_socat_in_guest() {
    limactl shell "$NAME" -- sh -c '
        command -v socat >/dev/null && exit 0
        if command -v apt-get >/dev/null; then sudo apt-get install -y socat
        elif command -v dnf >/dev/null; then sudo dnf install -y socat
        elif command -v apk >/dev/null; then sudo apk add socat
        elif command -v pacman >/dev/null; then sudo pacman -Sy --noconfirm socat
        elif command -v zypper >/dev/null; then sudo zypper in -y socat
        else echo "do not know how to install socat in the guest" >&2; exit 1
        fi'
}

# Returns the IPv4 address of the host LAN interface.
host_lan_ip() {
    local cache=$BATS_FILE_TMPDIR/host_lan_ip ip=""
    if [[ ! -s $cache ]]; then
        if [[ $OSTYPE == darwin* ]]; then
            # macOS GitHub runners use "localhost" as the hostname, so ask system_profiler instead.
            ip=$(system_profiler SPNetworkDataType -json |
                jq -r 'first(.SPNetworkDataType[] | select(.ip_address) | .ip_address) | first') || ip=""
            [[ $ip == null ]] && ip=""
        else
            ip=$(getent ahostsv4 "$(hostname)" 2>/dev/null | awk '{print $1; exit}') || ip=""
        fi
        echo "${ip:-127.0.0.1}" >"$cache"
    fi
    cat "$cache"
}

instance_dir() {
    local cache=$BATS_FILE_TMPDIR/instance_dir
    if [[ ! -s $cache ]]; then
        limactl list --format '{{.Dir}}' "$NAME" >"$cache"
    fi
    cat "$cache"
}

# The hostagent runs unprivileged and can bind host ports below 1024 only on macOS.
skip_unless_host_can_bind_privileged_ports() {
    if [[ $OSTYPE != darwin* ]]; then
        skip "the hostagent cannot bind host ports below 1024 on this host OS"
    fi
}

skip_if_wsl2() {
    if [[ $(limactl list --format '{{.VMType}}' "$NAME") == wsl2 ]]; then
        skip "$1"
    fi
}

skip_if_windows_host() {
    case $OSTYPE in
    cygwin | msys) skip "UNIX sockets are not supported on Windows hosts" ;;
    esac
}

# Format an address the way Go's net.JoinHostPort() does, e.g. "127.0.0.1:80" or "[::1]:80".
join_host_port() {
    local ip=$1 port=$2
    [[ $ip == *:* ]] && ip="[$ip]"
    echo "$ip:$port"
}

# The socat address to connect to ip:port from the host.
tcp_dest() {
    local ip=$1 port=$2
    if [[ $ip == *:* ]]; then
        echo "TCP6:[$ip]:$port,connect-timeout=5"
    else
        echo "TCP4:$ip:$port,connect-timeout=5"
    fi
}

guest_outfile() {
    echo "/tmp/port-forwarding.$1.out"
}

# Start a listener inside the guest that writes everything it receives to a
# file. socat exits after the first connection is closed. Ports below 1024
# require root to bind.
start_listener_in_guest() {
    local ip=$1 port=$2 proto=TCP sudo=""
    [[ $ip == *:* ]] && proto=TCP6
    ((port < 1024)) && sudo=sudo
    limactl shell "$NAME" -- sh -c \
        "nohup $sudo socat -u $proto-LISTEN:$port,bind=$ip STDOUT >$(guest_outfile "$port") 2>/dev/null </dev/null &"
}

# Wait until the hostagent reports a port-forwarding event of the given type
# ("forwarding" or "not-forwarding") for the guest address, and print it.
wait_for_port_forwarding_event() {
    local type=$1 guest_addr=$2 event
    local deadline=$((SECONDS + EVENT_TIMEOUT))
    while ((SECONDS < deadline)); do
        # The last line of the events file may still be incomplete; ignore jq failures.
        event=$(jq -n --arg type "$type" --arg addr "$guest_addr" \
            'first(inputs | .event.status.portForward | select(.type == $type and .guestAddr == $addr))' \
            "$BATS_FILE_TMPDIR/events" 2>/dev/null) || event=""
        if [[ -n $event ]]; then
            echo "$event"
            return 0
        fi
        sleep 0.5
    done
    fail "timed out waiting for the hostagent to report a '$type' event for $guest_addr"
}

send_from_host() {
    local message=$1 dest=$2
    socat -u - "$dest" <<<"$message"
}

# retry COUNT CMD [ARGS...]: retry a command with a 1 second pause until it succeeds.
retry() {
    local count=$1 i
    shift
    for ((i = 1; i < count; i++)); do
        if "$@"; then return 0; fi
        sleep 1
    done
    "$@"
}

read_guest_outfile() {
    # The guest path must be embedded in the command string: a bare /tmp/... argument
    # would be rewritten to C:/msys64/tmp/... by MSYS2 path conversion on Windows hosts.
    limactl shell "$NAME" -- sh -c "cat $(guest_outfile "$1")"
}

# Assert that the guest listener on the given port has received exactly the
# expected message. A non-empty message travels asynchronously, so keep polling
# while the listener output file is still empty.
assert_guest_received() {
    local port=$1 expected=$2 received
    local deadline=$((SECONDS + RECEIVE_TIMEOUT))
    received=$(read_guest_outfile "$port")
    while [[ -n $expected && -z $received ]] && ((SECONDS < deadline)); do
        sleep 0.5
        received=$(read_guest_outfile "$port")
    done
    assert_equal "$received" "$expected"
}

# Assert that a listener on GUEST_IP:GUEST_PORT is forwarded to HOST_IP:HOST_PORT:
# the hostagent must report a "forwarding" event to the expected host address,
# and a message sent to the host address must arrive at the guest listener.
assert_forwarded() {
    local guest_ip=$1 guest_port=$2 host_ip=$3 host_port=$4
    local guest_addr host_addr event message
    guest_addr=$(join_host_port "$guest_ip" "$guest_port")
    host_addr=$(join_host_port "$host_ip" "$host_port")
    message="message from host $host_addr to guest $guest_addr"

    start_listener_in_guest "$guest_ip" "$guest_port"
    event=$(wait_for_port_forwarding_event forwarding "$guest_addr")
    assert_equal "$(jq -r .hostAddr <<<"$event")" "$host_addr"

    # The "forwarding" event is emitted just before the hostagent binds the
    # host address, so the first connection attempt may be too early.
    retry 5 send_from_host "$message" "$(tcp_dest "$host_ip" "$host_port")"
    assert_guest_received "$guest_port" "$message"
}

# Assert that a listener on GUEST_IP:GUEST_PORT is NOT forwarded: the hostagent
# must report a "not-forwarding" event, and a message sent to the host address
# that would have been used must not arrive at the guest listener.
assert_not_forwarded() {
    local guest_ip=$1 guest_port=$2 host_ip=$3 host_port=$4
    local guest_addr
    guest_addr=$(join_host_port "$guest_ip" "$guest_port")

    start_listener_in_guest "$guest_ip" "$guest_port"
    wait_for_port_forwarding_event not-forwarding "$guest_addr" >/dev/null

    # Something else may be listening on the host address, so ignore connection errors.
    send_from_host "unexpected message to guest $guest_addr" "$(tcp_dest "$host_ip" "$host_port")" || :
    assert_guest_received "$guest_port" ""
}

# Like assert_forwarded, but the forwarding target is a UNIX socket in the
# "sock" subdirectory of the instance directory.
assert_forwarded_to_socket() {
    local guest_ip=$1 guest_port=$2 socket=$3
    local guest_addr socket_path event message
    guest_addr=$(join_host_port "$guest_ip" "$guest_port")
    socket_path=$(instance_dir)/sock/$socket
    message="message from host socket $socket to guest $guest_addr"

    start_listener_in_guest "$guest_ip" "$guest_port"
    event=$(wait_for_port_forwarding_event forwarding "$guest_addr")
    assert_equal "$(jq -r .hostAddr <<<"$event")" "$socket_path"

    retry 5 send_from_host "$message" "UNIX-CONNECT:$socket_path"
    assert_guest_received "$guest_port" "$message"
}

# Like assert_not_forwarded, but for a UNIX socket target: the socket must not
# have been created on the host.
assert_not_forwarded_to_socket() {
    local guest_ip=$1 guest_port=$2 socket=$3
    local guest_addr
    guest_addr=$(join_host_port "$guest_ip" "$guest_port")

    start_listener_in_guest "$guest_ip" "$guest_port"
    wait_for_port_forwarding_event not-forwarding "$guest_addr" >/dev/null

    assert_socket_not_exists "$(instance_dir)/sock/$socket"
    assert_guest_received "$guest_port" ""
}

# ---------------------------------------------------------------------------
# An ignore rule only applies to listeners matching its exact guestIP; other
# addresses are still forwarded by later rules. An ignore rule for 0.0.0.0
# with guestIPMustBeZero false blocks listeners on ALL addresses.

@test 'ignore 127.0.0.2:3000 (exact match of an ignore rule)' {
    assert_not_forwarded 127.0.0.2 3000 127.0.0.1 2000
}

@test 'forward 127.0.0.3:3001 -> 127.0.0.1:2001 (ignore rule for 127.0.0.2 does not match 127.0.0.3)' {
    assert_forwarded 127.0.0.3 3001 127.0.0.1 2001
}

@test 'forward 0.0.0.0:3002 -> 127.0.0.1:2002 (ignore rule for 127.0.0.2 cannot block a wildcard listener)' {
    assert_forwarded 0.0.0.0 3002 127.0.0.1 2002
}

@test 'ignore 0.0.0.0:3010 (exact match of the ignore rule for 0.0.0.0)' {
    skip_if_wsl2 'ignore rules for localhost ports cannot be enforced on WSL2'
    assert_not_forwarded 0.0.0.0 3010 127.0.0.1 2010
}

@test 'ignore 127.0.0.1:3011 (ignore rule for 0.0.0.0 blocks all addresses when guestIPMustBeZero is false)' {
    skip_if_wsl2 'ignore rules for localhost ports cannot be enforced on WSL2'
    assert_not_forwarded 127.0.0.1 3011 127.0.0.1 2011
}

# The forward rule for 0.0.0.0 [3000-3029] matches listeners on any address,
# including IPv6, and shadows the ignore rule for [3020-3029] that follows it.

@test 'forward 127.0.0.2:3020 -> 127.0.0.1:2020 (earlier forward rule shadows later ignore rule)' {
    assert_forwarded 127.0.0.2 3020 127.0.0.1 2020
}

@test 'forward 127.0.0.1:3021 -> 127.0.0.1:2021' {
    assert_forwarded 127.0.0.1 3021 127.0.0.1 2021
}

@test 'forward 0.0.0.0:3022 -> 127.0.0.1:2022' {
    assert_forwarded 0.0.0.0 3022 127.0.0.1 2022
}

@test 'forward [::]:3023 -> 127.0.0.1:2023 (wildcard rule also matches IPv6 listeners)' {
    assert_forwarded :: 3023 127.0.0.1 2023
}

@test 'forward [::1]:3024 -> 127.0.0.1:2024' {
    assert_forwarded ::1 3024 127.0.0.1 2024
}

# ---------------------------------------------------------------------------
# Forwarding to the host LAN address instead of localhost.

@test 'forward 127.0.0.1:3030 -> HOST_LAN_IP:2030' {
    assert_forwarded 127.0.0.1 3030 "$(host_lan_ip)" 2030
}

@test 'forward 0.0.0.0:3031 -> HOST_LAN_IP:2031' {
    assert_forwarded 0.0.0.0 3031 "$(host_lan_ip)" 2031
}

@test 'forward [::]:3032 -> HOST_LAN_IP:2032' {
    assert_forwarded :: 3032 "$(host_lan_ip)" 2032
}

@test 'forward [::1]:3033 -> HOST_LAN_IP:2033' {
    assert_forwarded ::1 3033 "$(host_lan_ip)" 2033
}

# ---------------------------------------------------------------------------
# Privileged host ports (below 1024). The default guestIP for a rule is
# 127.0.0.1, which also matches wildcard and IPv6 loopback listeners, but not
# the user-mode network address.

@test 'forward 127.0.0.1:300 -> 127.0.0.1:300 (privileged host port)' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 127.0.0.1 300 127.0.0.1 300
}

@test 'forward 0.0.0.0:301 -> 127.0.0.1:301' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 0.0.0.0 301 127.0.0.1 301
}

@test 'forward [::]:302 -> 127.0.0.1:302' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded :: 302 127.0.0.1 302
}

@test 'forward [::1]:303 -> 127.0.0.1:303' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded ::1 303 127.0.0.1 303
}

@test 'ignore 192.168.5.15:304 (default guestIP 127.0.0.1 does not match the user-mode network address)' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_not_forwarded 192.168.5.15 304 127.0.0.1 304
}

# Same as the previous rule, but with an explicit "guestIPMustBeZero: false".

@test 'forward 127.0.0.1:305 -> 127.0.0.1:305 (privileged host port)' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 127.0.0.1 305 127.0.0.1 305
}

@test 'forward 0.0.0.0:306 -> 127.0.0.1:306' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 0.0.0.0 306 127.0.0.1 306
}

@test 'forward [::]:307 -> 127.0.0.1:307' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded :: 307 127.0.0.1 307
}

@test 'forward [::1]:308 -> 127.0.0.1:308' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded ::1 308 127.0.0.1 308
}

@test 'ignore 192.168.5.15:309' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_not_forwarded 192.168.5.15 309 127.0.0.1 309
}

# hostIP 0.0.0.0 binds the forward on all host interfaces.

@test 'forward 127.0.0.1:310 -> 0.0.0.0:310 (hostIP 0.0.0.0 binds all host interfaces)' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 127.0.0.1 310 0.0.0.0 310
}

@test 'forward 0.0.0.0:311 -> 0.0.0.0:311' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 0.0.0.0 311 0.0.0.0 311
}

@test 'forward [::]:312 -> 0.0.0.0:312' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded :: 312 0.0.0.0 312
}

@test 'forward [::1]:313 -> 0.0.0.0:313' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded ::1 313 0.0.0.0 313
}

@test 'ignore 192.168.5.15:314' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_not_forwarded 192.168.5.15 314 0.0.0.0 314
}

# Same as the previous rule, but with an explicit "guestIPMustBeZero: false".

@test 'forward 127.0.0.1:315 -> 0.0.0.0:315 (privileged host port)' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 127.0.0.1 315 0.0.0.0 315
}

@test 'forward 0.0.0.0:316 -> 0.0.0.0:316' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded 0.0.0.0 316 0.0.0.0 316
}

@test 'forward [::]:317 -> 0.0.0.0:317' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded :: 317 0.0.0.0 317
}

@test 'forward [::1]:318 -> 0.0.0.0:318' {
    skip_unless_host_can_bind_privileged_ports
    assert_forwarded ::1 318 0.0.0.0 318
}

@test 'ignore 192.168.5.15:319' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_not_forwarded 192.168.5.15 319 0.0.0.0 319
}

# ---------------------------------------------------------------------------
# guestIP set to the user-mode network address of the guest.

@test 'forward 192.168.5.15:4000 -> HOST_LAN_IP:4000 (guestIP is the user-mode network address)' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_forwarded 192.168.5.15 4000 "$(host_lan_ip)" 4000
}

# ---------------------------------------------------------------------------
# IPv6 guestIP and hostIP.

@test 'forward [::1]:4010 -> [::]:4010 (IPv6 guestIP and hostIP)' {
    assert_forwarded ::1 4010 :: 4010
}

# ---------------------------------------------------------------------------
# guestIP "::" (with the default guestIPMustBeZero false) matches listeners on
# any address.

@test 'forward 127.0.0.1:4020 -> HOST_LAN_IP:4020 (guestIP :: matches any listener address)' {
    assert_forwarded 127.0.0.1 4020 "$(host_lan_ip)" 4020
}

@test 'forward 127.0.0.2:4021 -> HOST_LAN_IP:4021' {
    assert_forwarded 127.0.0.2 4021 "$(host_lan_ip)" 4021
}

@test 'forward 192.168.5.15:4022 -> HOST_LAN_IP:4022' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_forwarded 192.168.5.15 4022 "$(host_lan_ip)" 4022
}

@test 'forward 0.0.0.0:4023 -> HOST_LAN_IP:4023' {
    assert_forwarded 0.0.0.0 4023 "$(host_lan_ip)" 4023
}

@test 'forward [::]:4024 -> HOST_LAN_IP:4024' {
    assert_forwarded :: 4024 "$(host_lan_ip)" 4024
}

@test 'forward [::1]:4025 -> HOST_LAN_IP:4025' {
    assert_forwarded ::1 4025 "$(host_lan_ip)" 4025
}

# guestIP "0.0.0.0" with guestIPMustBeZero false behaves exactly like "::".

@test 'forward 127.0.0.1:4030 -> HOST_LAN_IP:4030 (guestIP 0.0.0.0 matches any listener address)' {
    assert_forwarded 127.0.0.1 4030 "$(host_lan_ip)" 4030
}

@test 'forward 127.0.0.2:4031 -> HOST_LAN_IP:4031' {
    assert_forwarded 127.0.0.2 4031 "$(host_lan_ip)" 4031
}

@test 'forward 192.168.5.15:4032 -> HOST_LAN_IP:4032' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_forwarded 192.168.5.15 4032 "$(host_lan_ip)" 4032
}

@test 'forward 0.0.0.0:4033 -> HOST_LAN_IP:4033' {
    assert_forwarded 0.0.0.0 4033 "$(host_lan_ip)" 4033
}

@test 'forward [::]:4034 -> HOST_LAN_IP:4034' {
    assert_forwarded :: 4034 "$(host_lan_ip)" 4034
}

@test 'forward [::1]:4035 -> HOST_LAN_IP:4035' {
    assert_forwarded ::1 4035 "$(host_lan_ip)" 4035
}

# ---------------------------------------------------------------------------
# With guestIPMustBeZero true only wildcard listeners match; listeners on any
# other address fall through to the next rule, which ignores them.

@test 'forward 0.0.0.0:4040 -> 127.0.0.1:4040 (wildcard listener matches guestIPMustBeZero rule)' {
    assert_forwarded 0.0.0.0 4040 127.0.0.1 4040
}

@test 'forward [::]:4041 -> 127.0.0.1:4041 (IPv6 wildcard listener matches guestIPMustBeZero rule)' {
    assert_forwarded :: 4041 127.0.0.1 4041
}

@test 'ignore 127.0.0.1:4043 (loopback listener does not match a guestIPMustBeZero rule)' {
    skip_if_wsl2 'ignore rules for localhost ports cannot be enforced on WSL2'
    assert_not_forwarded 127.0.0.1 4043 127.0.0.1 4043
}

@test 'ignore 192.168.5.15:4044' {
    skip_if_wsl2 'the guest has no user-mode network address on WSL2'
    assert_not_forwarded 192.168.5.15 4044 127.0.0.1 4044
}

# ---------------------------------------------------------------------------
# Forwarding to a UNIX socket on the host.

@test 'forward 127.0.0.1:5000 -> sock/port5000.sock' {
    skip_if_windows_host
    assert_forwarded_to_socket 127.0.0.1 5000 port5000.sock
}

@test 'ignore 192.168.5.15:5001 (no socket is created for an unmatched listener)' {
    skip_if_windows_host
    assert_not_forwarded_to_socket 192.168.5.15 5001 port5001.sock
}

@test 'forward 127.0.0.1:5002 -> sock/port5002.sock (guestIPMustBeZero false)' {
    skip_if_windows_host
    assert_forwarded_to_socket 127.0.0.1 5002 port5002.sock
}

@test 'ignore 192.168.5.15:5003 (no socket is created for an unmatched listener)' {
    skip_if_windows_host
    assert_not_forwarded_to_socket 192.168.5.15 5003 port5003.sock
}
