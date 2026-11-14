# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Tests for the `meta` field, which holds annotations that Lima itself never reads.
# The merge rules for `base` templates and default.yaml/override.yaml are covered by
# unit tests in pkg/limatmpl and pkg/limayaml.

load "../helpers/load"

INSTANCE=bats-dummy
CLONE=bats-meta-clone

local_setup_file() {
    limactl delete --force "$CLONE" || :
}

local_teardown_file() {
    limactl delete --force "$CLONE" || :
}

@test 'meta can be set with limactl edit without a non-strict YAML warning' {
    run_e -0 limactl edit --tty=false "$INSTANCE" \
        --set '.meta.database = true | .meta.owner = {"manager": "tool", "locked": ["delete", "edit"]}'
    refute_stderr --partial 'Non-strict YAML'
}

@test 'limactl list can select instances by meta' {
    run_e -0 limactl list --yq 'select(.config.meta.database).name'
    assert_line "$INSTANCE"
}

@test 'limactl list exposes nested meta as JSON' {
    run_e -0 limactl list --format '{{json .Config.Meta}}' "$INSTANCE"
    assert_output '{"database":true,"owner":{"locked":["delete","edit"],"manager":"tool"}}'
}

@test 'limactl clone copies meta to the new instance' {
    run_e -0 limactl clone --tty=false "$INSTANCE" "$CLONE"
    run_e -0 limactl list --yq '.config.meta.owner.manager' "$CLONE"
    assert_output 'tool'
}

@test 'meta can be removed with limactl edit' {
    run_e -0 limactl edit --tty=false "$INSTANCE" --set 'del(.meta)'
    run_e -0 limactl list --format '{{json .Config.Meta}}' "$INSTANCE"
    assert_output 'null'
}
