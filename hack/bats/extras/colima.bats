# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

local_setup_file() {
    colima start
}

local_teardown_file() {
    colima stop
    colima delete -f
}

@test 'Docker' {
    docker run -p 8080:80 -d --name nginx "${TEST_CONTAINER_IMAGES[nginx]}"
    sleep 5
    run -0 curl -sSI --retry 5 --retry-all-errors http://localhost:8080
    assert_output --partial "200 OK"
    docker rm -f nginx
}
