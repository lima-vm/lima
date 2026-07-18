# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This test verifies that when a container is destroyed, its ports can be reused
# immediately and are not subject to being freed by a polling loop. See #4066.

# This test should not be run in CI as it is not totally reliable: there is always a chance that the server will
# take longer to actually respond to requests after opening the port. The test works around it by retrying once
# on curl exit code 52, but have been observed at least once to fail by refusing to connect.

load "../helpers/load"

: "${TEMPLATE:=default}" # Alternative: "docker"

NAME=nginx

local_setup_file() {
    limactl delete --force "$NAME" || :
    limactl start --yes --name "$NAME" --mount "$BATS_TMPDIR" "template:${TEMPLATE}" 3>&- 4>&-
}

local_teardown_file() {
    limactl delete --force "$NAME"
}

ctrctl() {
    if [[ $(limactl ls "$NAME" --yq .config.containerd.user) == true ]]; then
        limactl shell $NAME nerdctl "$@"
    else
        limactl shell $NAME docker "$@"
    fi
}

nginx_start() {
    echo "$COUNTER" >"${BATS_TEST_TMPDIR}/index.html"
    ctrctl run -d --name nginx -p 8080:80 -v "${BATS_TEST_TMPDIR}:/usr/share/nginx/html:ro" nginx
}

nginx_stop() {
    ctrctl stop nginx
    ctrctl rm nginx
}

verify_port() {
    run curl --silent http://127.0.0.1:8080
    # If nginx is not quite ready and doesn't send any response at all, give it one extra chance
    if [[ $status -eq 52 ]]; then
        sleep 0.5
        run curl --silent http://127.0.0.1:8080
    fi
    assert_success
    assert_output "$COUNTER"
}

@test 'Verify that the container is working' {
    COUNTER=0
    ctrctl pull --quiet nginx
    nginx_start
    verify_port
}

@test 'Stop and restart the container multiple times' {
    for COUNTER in {1..100}; do
        nginx_stop
        nginx_start
        verify_port
    done
}
