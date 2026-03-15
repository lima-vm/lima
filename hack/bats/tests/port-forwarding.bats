# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This test verifies Lima's port forwarding rules by starting listeners in the guest,
# sending data from the host, and checking both event logs and data delivery.
#
# Usage:
#   bats --formatter tap hack/bats/tests/port-forwarding.bats
#     Creates and deletes its own "bats-portfwd" instance.
#
#   LIMA_BATS_PORTFWD_INSTANCE=default bats --formatter tap hack/bats/tests/port-forwarding.bats
#     Reuses an existing instance (skips create/delete).
#
# Environment variables:
#   LIMA_BATS_PORTFWD_INSTANCE  - Use an existing instance instead of creating one.
#   LIMA_BATS_PORTFWD_TIMEOUT   - Connection timeout in seconds (default: 1).
#   LIMA_BATS_REUSE_INSTANCE    - When set, skip instance deletion on teardown.

load "../helpers/load"

if [[ -n ${LIMA_BATS_PORTFWD_INSTANCE:-} ]]; then
	NAME="$LIMA_BATS_PORTFWD_INSTANCE"
else
	NAME="bats-portfwd"
	INSTANCE="$NAME"
fi
CONNECTION_TIMEOUT=${LIMA_BATS_PORTFWD_TIMEOUT:-1}

# Source the shared port forwarding config (YAML rules + get_host_ipv4).
# shellcheck source=../../port-forwarding-config.sh
source "$(cd "$(dirname "${BATS_TEST_FILENAME}")" && pwd)/../../port-forwarding-config.sh"

join_host_port() {
	local ip=$1 port=$2
	if [[ $ip == *:* ]]; then
		echo "[$ip]:$port"
	else
		echo "$ip:$port"
	fi
}

parse_test_cases() {
	local host_ipv4=$1 sock_dir=$2 ssh_local_port=$3
	while IFS= read -r line; do
		# Skip non-test lines
		[[ $line =~ ^(forward|ignore): ]] || continue
		# Replace placeholders
		line="${line//HOST_IPV4/$host_ipv4}"
		line="${line//SOCK_DIR\//$sock_dir}"
		line="${line//SSH_LOCAL_PORT/$ssh_local_port}"

		local mode="" guest_ip="" guest_port="" host_ip="" host_port="" host_socket=""
		if [[ $line =~ ^(forward|ignore):[[:space:]]+([0-9.:]+)[[:space:]]+([0-9]+)([[:space:]]+-\>[[:space:]]+(.+))? ]]; then
			mode="${BASH_REMATCH[1]}"
			guest_ip="${BASH_REMATCH[2]}"
			guest_port="${BASH_REMATCH[3]}"
			local rest="${BASH_REMATCH[5]}"
			if [[ -n $rest ]]; then
				if [[ $rest == */* ]]; then
					# Unix socket path
					host_socket="$rest"
				elif [[ $rest =~ ^([0-9.:]+)[[:space:]]+([0-9]+)$ ]]; then
					host_ip="${BASH_REMATCH[1]}"
					host_port="${BASH_REMATCH[2]}"
				fi
			fi
			# Defaults for non-socket cases
			if [[ -z $host_socket ]]; then
				host_ip="${host_ip:-127.0.0.1}"
				host_port="${host_port:-$guest_port}"
			fi
			printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$mode" "$guest_ip" "$guest_port" "${host_ip:--}" "${host_port:--}" "${host_socket:--}"
		fi
	done < <(port_forwards_block | sed -n 's/^[[:space:]]*#[[:space:]]*//p')
}

normalize_fields() {
	if [[ $host_ip == "-" ]]; then host_ip=""; fi
	if [[ $host_port == "-" ]]; then host_port=""; fi
	if [[ $host_socket == "-" ]]; then host_socket=""; fi
}

build_log_msg() {
	local mode=$1 guest_ip=$2 guest_port=$3 host_ip=$4 host_port=$5 host_socket=$6
	local remote
	remote=$(join_host_port "$guest_ip" "$guest_port")
	if [[ $mode == "forward" ]]; then
		local local_addr
		if [[ -n $host_socket ]]; then
			local_addr="$host_socket"
		else
			local_addr=$(join_host_port "$host_ip" "$host_port")
		fi
		echo "Forwarding TCP from $remote to $local_addr"
	else
		echo "Not forwarding TCP $remote"
	fi
}

skip_reason() {
	local mode=$1 guest_ip=$2 guest_port=$3 host_ip=$4 host_port=$5 host_socket=$6

	if [[ $guest_ip == *:* ]] && ! ip -6 addr show lo >/dev/null 2>&1; then
		echo "Not yet"
		return 0
	fi
	if [[ $mode == "forward" && -z $host_socket && $host_port -lt 1024 && "$(uname -s)" != "Darwin" ]]; then
		echo "Not supported on $(uname -s | tr '[:upper:]' '[:lower:]')"
		return 0
	fi
	if [[ $VM_TYPE == "wsl2" ]]; then
		if [[ $mode == "ignore" && ($guest_ip == "0.0.0.0" || $guest_ip == "127.0.0.1") ]]; then
			echo "Not supported for $VM_TYPE machines"
			return 0
		fi
		if [[ $guest_ip == "192.168.5.15" ]]; then
			echo "Not supported for $VM_TYPE machines"
			return 0
		fi
	fi
	case "$(uname -o 2>/dev/null)" in
	Cygwin | Msys)
		if [[ -n $host_socket ]]; then
			echo "Not supported on Windows"
			return 0
		fi
		;;
	esac
	return 1
}

assert_watch_event() {
	local type=$1 guest_addr=$2 host_addr=${3:-}
	local filter
	if [[ -n $host_addr ]]; then
		filter="select(.event.status.portForward.type == \"$type\"
            and .event.status.portForward.guestAddr == \"$guest_addr\"
            and .event.status.portForward.hostAddr == \"$host_addr\")"
	else
		filter="select(.event.status.portForward.type == \"$type\"
            and .event.status.portForward.guestAddr == \"$guest_addr\")"
	fi
	jq -e "$filter" "$EVENTS_FILE" >/dev/null 2>&1
}

# check_test_cases runs assertions for test cases matching the given port range.
check_test_cases() {
	local min_port=$1 max_port=$2
	local id=0 failures=0
	while IFS=$'\t' read -r mode guest_ip guest_port host_ip host_port host_socket; do
		[[ -n $mode ]] || continue
		normalize_fields

		if [[ $guest_port -lt $min_port || $guest_port -gt $max_port ]]; then
			id=$((id + 1))
			continue
		fi

		local log_msg
		log_msg=$(build_log_msg "$mode" "$guest_ip" "$guest_port" "$host_ip" "$host_port" "$host_socket")

		local reason
		if reason=$(skip_reason "$mode" "$guest_ip" "$guest_port" "$host_ip" "$host_port" "$host_socket"); then
			echo "# skipped ($reason): $log_msg" >&3
			id=$((id + 1))
			continue
		fi

		local remote
		remote=$(join_host_port "$guest_ip" "$guest_port")

		local event_err="" data_err=""
		if [[ -s $EVENTS_FILE ]]; then
			if [[ $mode == "forward" ]]; then
				if ! assert_watch_event "forwarding" "$remote" "${host_socket:-$(join_host_port "$host_ip" "$host_port")}"; then
					event_err="Event missing from watch --json output"
				fi
			else
				if ! assert_watch_event "not-forwarding" "$remote"; then
					event_err="Event missing from watch --json output"
				fi
			fi
		fi
		if [[ $host_port != "$SSH_LOCAL_PORT" ]]; then
			local got
			got=$(tr -d '\r' <"$RESULTS_DIR/socat.${id}" 2>/dev/null | sed '/^$/d')
			if [[ $mode == "forward" && $got != "$log_msg" ]]; then
				data_err="Guest received: '${got}'"
			fi
			if [[ $mode == "ignore" && -n $got ]]; then
				data_err="Guest received: '${got}' (instead of nothing)"
			fi
		fi

		if [[ -n $data_err ]]; then
			local full_err=""
			[[ -z $event_err ]] || full_err=$'\n'"  $event_err"
			full_err="${full_err}"$'\n'"  $data_err"
			echo "# FAIL: $log_msg$full_err" >&3
			failures=$((failures + 1))
		elif [[ -n $event_err ]]; then
			echo "# warning: $log_msg - $event_err" >&3
		else
			echo "# ok: $log_msg" >&3
		fi
		id=$((id + 1))
	done <"$BATS_FILE_TMPDIR/test_cases.tsv"

	[[ $failures -eq 0 ]]
}

local_setup_file() {
	HOST_IPV4=$(get_host_ipv4)
	export HOST_IPV4

	EVENTS_FILE="$BATS_FILE_TMPDIR/events.jsonl"
	export EVENTS_FILE
	RESULTS_DIR="$BATS_FILE_TMPDIR/results"
	mkdir -p "$RESULTS_DIR"
	export RESULTS_DIR

	local inst_dir
	inst_dir=$(limactl list "$NAME" --yq .dir)
	SOCK_DIR="${inst_dir}/sock/"
	export SOCK_DIR

	SSH_LOCAL_PORT=$(limactl ls --json "$NAME" | jq -r '.sshLocalPort')
	export SSH_LOCAL_PORT

	VM_TYPE=$(limactl ls --json "$NAME" | jq -r '.vmType')
	export VM_TYPE

	if limactl shell "$NAME" command -v apt-get >/dev/null 2>&1; then
		limactl shell "$NAME" sudo apt-get install -y socat >/dev/null 2>&1
	elif limactl shell "$NAME" command -v apk >/dev/null 2>&1; then
		limactl shell "$NAME" sudo apk add socat >/dev/null 2>&1
	elif limactl shell "$NAME" command -v dnf >/dev/null 2>&1; then
		limactl shell "$NAME" sudo dnf install -y socat >/dev/null 2>&1
	elif limactl shell "$NAME" command -v pacman >/dev/null 2>&1; then
		limactl shell "$NAME" sudo pacman -Syu --noconfirm socat >/dev/null 2>&1
	elif limactl shell "$NAME" command -v zypper >/dev/null 2>&1; then
		limactl shell "$NAME" sudo zypper in -y socat >/dev/null 2>&1
	fi

	limactl shell "$NAME" bash -c 'sudo pkill -x socat 2>/dev/null || true; rm -f ~/socat.*'
	sleep 5

	local test_cases
	test_cases=$(parse_test_cases "$HOST_IPV4" "$SOCK_DIR" "$SSH_LOCAL_PORT")
	echo "$test_cases" >"$BATS_FILE_TMPDIR/test_cases.tsv"

	timeout 120 limactl watch --json "$NAME" >"$EVENTS_FILE" 2>/dev/null 3>&- 4>&- &
	WATCH_PID=$!
	export WATCH_PID
	sleep 1

	local listener_script="$BATS_FILE_TMPDIR/start-listeners.sh"
	{
		echo '#!/bin/bash'
		echo 'cd $HOME'
		echo 'rm -f socat.*'
		local id=0
		while IFS=$'\t' read -r mode guest_ip guest_port host_ip host_port host_socket; do
			[[ -n $mode ]] || continue
			normalize_fields
			local proto="TCP"
			if [[ $guest_ip == *:* ]]; then proto="TCP6"; fi
			local sudo=""
			if [[ $guest_port -lt 1024 ]]; then sudo="sudo "; fi
			echo "${sudo}socat -u ${proto}-LISTEN:${guest_port},bind=${guest_ip},reuseaddr OPEN:\$HOME/socat.${id},creat </dev/null >/dev/null 2>&1 &"
			id=$((id + 1))
		done <"$BATS_FILE_TMPDIR/test_cases.tsv"
	} >"$listener_script"

	local listener_script_host="$listener_script"
	if command -v cygpath >/dev/null 2>&1; then
		listener_script_host="$(cygpath -w "$listener_script")"
	fi
	limactl cp "$listener_script_host" "${NAME}:/tmp/start-listeners.sh"
	limactl shell "$NAME" bash /tmp/start-listeners.sh </dev/null

	# Count expected forwarding events (excluding SSH port and skipped cases)
	local expected_fwd=0
	while IFS=$'\t' read -r mode guest_ip guest_port host_ip host_port host_socket; do
		[[ -n $mode ]] || continue
		normalize_fields
		[[ $mode == "forward" ]] || continue
		[[ $host_port != "$SSH_LOCAL_PORT" ]] || continue
		skip_reason "$mode" "$guest_ip" "$guest_port" "$host_ip" "$host_port" "$host_socket" >/dev/null && continue
		expected_fwd=$((expected_fwd + 1))
	done <"$BATS_FILE_TMPDIR/test_cases.tsv"

	# Wait for all forwarding events to appear (up to 15s)
	local wait_deadline=$((SECONDS + 15))
	while [[ $SECONDS -lt $wait_deadline ]]; do
		local got_fwd=0
		if [[ -s $EVENTS_FILE ]]; then
			got_fwd=$(jq -s '[.[] | select(.event.status.portForward.type == "forwarding")] | length' "$EVENTS_FILE" 2>/dev/null || echo 0)
		fi
		if [[ $got_fwd -ge $expected_fwd ]]; then
			break
		fi
		sleep 1
	done
	# Pause for host-side listeners to finish starting after events are emitted.
	# The hostagent emits the forwarding event before spawning the listener goroutine,
	# so we need to wait for the actual listeners to be ready.
	sleep 5

	id=0
	while IFS=$'\t' read -r mode guest_ip guest_port host_ip host_port host_socket; do
		[[ -n $mode ]] || continue
		normalize_fields

		local msg
		msg=$(build_log_msg "$mode" "$guest_ip" "$guest_port" "$host_ip" "$host_port" "$host_socket")

		if [[ $host_port == "$SSH_LOCAL_PORT" ]]; then
			id=$((id + 1))
			continue
		fi

		if skip_reason "$mode" "$guest_ip" "$guest_port" "$host_ip" "$host_port" "$host_socket" >/dev/null; then
			id=$((id + 1))
			continue
		fi

		local socat_cmd
		if [[ -n $host_socket ]]; then
			socat_cmd="socat -u STDIN UNIX-CONNECT:${host_socket}"
		elif [[ $host_ip == *:* ]]; then
			socat_cmd="socat -u STDIN TCP6:[${host_ip}]:${host_port},connect-timeout=${CONNECTION_TIMEOUT}"
		else
			socat_cmd="socat -u STDIN TCP:${host_ip}:${host_port},connect-timeout=${CONNECTION_TIMEOUT}"
		fi
		echo "$msg" | $socat_cmd 2>/dev/null || true

		id=$((id + 1))
	done <"$BATS_FILE_TMPDIR/test_cases.tsv"

	sleep 3
	kill "$WATCH_PID" 2>/dev/null || true
	wait "$WATCH_PID" 2>/dev/null || true

	local num_cases
	num_cases=$(wc -l <"$BATS_FILE_TMPDIR/test_cases.tsv")
	local fetch_script='cd $HOME'
	for ((i = 0; i < num_cases; i++)); do
		fetch_script="${fetch_script}; echo '===${i}==='; cat socat.${i} 2>/dev/null || true"
	done
	local all_output
	all_output=$(limactl shell --workdir / "$NAME" bash -c "$fetch_script" 2>/dev/null) || true
	local current_id=""
	while IFS= read -r line; do
		if [[ $line =~ ^===([0-9]+)=== ]]; then
			current_id="${BASH_REMATCH[1]}"
			: >"$RESULTS_DIR/socat.${current_id}"
		elif [[ -n $current_id ]]; then
			echo "$line" >>"$RESULTS_DIR/socat.${current_id}"
		fi
	done <<<"$all_output"
}

local_teardown_file() {
	limactl shell "$NAME" bash -c 'sudo pkill -x socat 2>/dev/null || true' 2>/dev/null || true
}

@test "ignore for specific guest IP" {
	check_test_cases 3000 3009
}

@test "ignore with guestIPMustBeZero false" {
	check_test_cases 3010 3019
}

@test "forward from any interface" {
	check_test_cases 3000 3029
}

@test "forward to host external IP" {
	check_test_cases 3030 3039
}

@test "forward privileged ports" {
	check_test_cases 300 304
}

@test "forward privileged ports guestIPMustBeZero false" {
	check_test_cases 305 309
}

@test "forward to hostIP 0.0.0.0" {
	check_test_cases 310 314
}

@test "forward to hostIP 0.0.0.0 guestIPMustBeZero false" {
	check_test_cases 315 319
}

@test "forward from specific guest IP" {
	check_test_cases 4000 4009
}

@test "forward IPv6 loopback" {
	check_test_cases 4010 4019
}

@test "forward from IPv6 any" {
	check_test_cases 4020 4029
}

@test "forward from IPv4 any guestIPMustBeZero false" {
	check_test_cases 4030 4039
}

@test "guestIPMustBeZero true with ignore fallback" {
	check_test_cases 4040 4049
}

@test "socket forwarding" {
	check_test_cases 5000 5003
}

@test "no unexpected or failed watch events" {
	[[ -s $EVENTS_FILE ]] || skip "no events collected"

	local -A expected_events
	while IFS=$'\t' read -r mode guest_ip guest_port host_ip host_port host_socket; do
		[[ -n $mode ]] || continue
		normalize_fields
		local r
		r=$(join_host_port "$guest_ip" "$guest_port")
		if [[ $mode == "forward" ]]; then
			local la
			if [[ -n $host_socket ]]; then la="$host_socket"; else la=$(join_host_port "$host_ip" "$host_port"); fi
			expected_events["forwarding:${r}:${la}"]=1
		else
			expected_events["not-forwarding:${r}:"]=1
		fi
	done <"$BATS_FILE_TMPDIR/test_cases.tsv"
	expected_events["forwarding:127.0.0.1:22:127.0.0.1:${SSH_LOCAL_PORT}"]=1

	local failures=0
	while IFS= read -r line; do
		local type guest_addr host_addr
		type=$(echo "$line" | jq -r '.event.status.portForward.type // empty')
		[[ -n $type ]] || continue
		[[ $type == "forwarding" || $type == "not-forwarding" ]] || continue
		guest_addr=$(echo "$line" | jq -r '.event.status.portForward.guestAddr // empty')
		host_addr=$(echo "$line" | jq -r '.event.status.portForward.hostAddr // empty')
		local key="${type}:${guest_addr}:${host_addr}"
		if [[ -z ${expected_events[$key]:-} ]]; then
			echo "# Unexpected: $type $guest_addr -> $host_addr" >&3
		fi
	done <"$EVENTS_FILE"

	# Check for failed forward events
	while IFS= read -r line; do
		local ftype ferror fhost_addr
		ftype=$(echo "$line" | jq -r '.event.status.portForward.type // empty')
		[[ $ftype == "failed" ]] || continue
		ferror=$(echo "$line" | jq -r '.event.status.portForward.error // empty')
		fhost_addr=$(echo "$line" | jq -r '.event.status.portForward.hostAddr // empty')
		echo "# Failed: $fhost_addr ($ferror)" >&3
		failures=$((failures + 1))
	done <"$EVENTS_FILE"

	[[ $failures -eq 0 ]]
}
