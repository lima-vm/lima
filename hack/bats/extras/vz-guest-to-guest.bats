# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests for `networks[].guestToGuest` with vzNAT.
#
# With the option set, the guest agent keeps a route that sends traffic for
# the vzNAT subnet through the gateway, and adds the route again when the
# network configuration removes it. Checking traffic between guests would need
# a second instance, but GitHub's macOS runners lose contact when two
# hardware-accelerated guests run at once. So these tests check the route in
# a single guest.
#
# The tests share one instance and must run in order.
#
# Host requirements: macOS.

load "../helpers/load"

NAME=bats-vz-guest-to-guest

local_setup_file() {
    [[ $(uname) == "Darwin" ]] || skip "requires macOS host"

    limactl unprotect "${NAME}" || :
    limactl delete --force "${NAME}" || :
    local template=${BATS_FILE_TMPDIR}/${NAME}.yaml
    cat >"${template}" <<'YAML'
base: template:default
vmType: vz
networks:
- vzNAT: true
  guestToGuest: true
YAML
    # LIMACTL_CREATE_ARGS is a list of arguments, so it must not be quoted.
    # shellcheck disable=SC2086
    limactl start --yes --name "${NAME}" ${LIMACTL_CREATE_ARGS:-} "${template}" 3>&- 4>&-
}

local_teardown_file() {
    delete_instance "${NAME}"
}

bats::on_failure() {
    if [[ -d ${LIMA_HOME}/${NAME} ]]; then
        tail -n 100 "${LIMA_HOME}/${NAME}"/*.log || :
    fi
}

# Assert that the only static route on lima0 is an onlink route via the
# gateway of the default route.
assert_guest_to_guest_route() {
    run -0 limactl shell "${NAME}" -- ip -4 route show dev lima0
    assert_line --regexp '^default via [0-9.]+ '
    [[ ${output} =~ default\ via\ ([0-9.]+)\  ]]
    local gateway=${BASH_REMATCH[1]}
    run -0 grep 'proto static' <<<"${output}"
    assert_output_lines_count 1
    assert_output --regexp "^[0-9./]+ via ${gateway//./\\.} proto static .* onlink"
}

# The guest agent logs "added guest-to-guest route" only when it adds a
# missing route, so the count is how often the agent found the route missing.
assert_agent_route_log_count() {
    run -0 limactl shell "${NAME}" -- sudo journalctl -u lima-guestagent -b --no-pager --output=cat
    # grep exits 1 when the count is 0, so the helper ignores the exit status
    # and compares only the count. A grep error still fails assert_output,
    # because run puts grep's error message in ${output}.
    run grep --count 'added guest-to-guest route' <<<"${output}"
    assert_output "$1"
}

@test "the guest agent adds the guest-to-guest route at boot" {
    assert_guest_to_guest_route
    assert_agent_route_log_count 1
}

@test "the guest agent adds the route again after networkctl reconfigure removes it" {
    limactl shell "${NAME}" -- sudo networkctl reconfigure lima0
    retry 30 assert_agent_route_log_count 2
    assert_guest_to_guest_route
}
