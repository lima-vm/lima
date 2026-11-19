# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests for `networks[].guestToGuest` with vzNAT.
#
# With the option set, a boot script adds a route that sends traffic for the
# vzNAT subnet through the gateway. On this Ubuntu guest, the script writes
# the route into a systemd-networkd drop-in, so networkd adds the route again
# whenever it reconfigures the interface. Checking traffic between guests
# would need a second instance, but GitHub's macOS runners lose contact when
# two hardware-accelerated guests run at once. So these tests check the route
# in a single guest.
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

# Assert that lima0 has exactly one static route. It must go to lima0's own
# subnet via the default route's gateway, with metric 99, one below the
# default network metric of 100.
assert_guest_to_guest_route() {
    run -0 --separate-stderr limactl shell "${NAME}" -- ip -4 route show dev lima0 || return
    [[ ${output} =~ default\ via\ ([0-9.]+)\  ]] || return
    local gateway=${BASH_REMATCH[1]}
    [[ ${output} =~ (^|$'\n')([0-9./]+)\ proto\ kernel ]] || return
    local subnet=${BASH_REMATCH[2]}
    run -0 grep 'proto static' <<<"${output}" || return
    assert_output_lines_count 1 || return
    assert_output --regexp "^${subnet//./\\.} via ${gateway//./\\.} proto static .*metric 99( |$)"
}

@test "the guest-to-guest route is there after boot" {
    assert_guest_to_guest_route
}

@test "networkd adds the deleted route back on networkctl reconfigure" {
    limactl shell "${NAME}" -- sudo ip -4 route flush dev lima0 proto static
    run -0 --separate-stderr limactl shell "${NAME}" -- ip -4 route show dev lima0 proto static
    refute_output
    limactl shell "${NAME}" -- sudo networkctl reconfigure lima0
    retry 30 assert_guest_to_guest_route
}
