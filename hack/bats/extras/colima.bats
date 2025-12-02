# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

@test 'Docker' {
    colima start
    docker run -p 8080:80 -d --name nginx "${TEST_CONTAINER_IMAGES[nginx]}"
    sleep 5
    run curl -sSI http://localhost:8080
    [ "$status" -eq 0 ]
    [[ "$output" == *"200 OK"* ]]
    docker rm -f nginx
    colima stop
    colima delete -f
}
