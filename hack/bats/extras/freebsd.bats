# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

local_setup_file() {
    limactl start --tty=false template:freebsd
}

local_teardown_file() {
    # <DEBUG>
    set -x
    tail -n 100 "${LIMA_HOME}"/freebsd/*.log
    limactl shell freebsd -- cat /var/log/messages
    # </DEBUG>
    limactl stop freebsd
    limactl rm freebsd
}

@test 'Smoke test' {
    run -0 limactl shell freebsd -- uname
    # FIXME: the shell always shows freebsd-tips (specified in ~/.login)
    assert_output --partial "FreeBSD"
}

@test 'Restart keeps the pinned SSH host key' {
    pinned="$(cat "${LIMA_HOME}/freebsd/ssh.known_hosts")"
    limactl stop freebsd
    limactl start --tty=false freebsd
    run -0 limactl shell freebsd -- true
    # The address and the port may change, but the key must not.
    assert_equal "$(awk '{print $2, $3}' "${LIMA_HOME}/freebsd/ssh.known_hosts")" "$(awk '{print $2, $3}' <<<"${pinned}")"
}
