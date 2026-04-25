# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

LOCAL_LIMA_HOME="${LIMA_HOME:?}/_bats_param"

local_setup_file() {
    export LIMA_HOME="${LOCAL_LIMA_HOME:?}"
    rm -rf "${LOCAL_LIMA_HOME:?}"
}

local_setup() {
    export LIMA_HOME="${LOCAL_LIMA_HOME:?}"
}

param_template() {
    cat <<'EOF'
images:
- location: /etc/profile
plain: true
param:
  release: ""
provision:
- mode: system
  script: |
    echo "$PARAM_release"
EOF
}

@test 'create accepts --param shortcut' {
    run -0 limactl create --name param-create --param=release=v1.35 - <<<"$(param_template)"

    run -0 limactl yq -r .param.release <"${LIMA_HOME}/param-create/lima.yaml"
    assert_output "v1.35"
}

@test 'create rejects undefined --param' {
    run_e -1 limactl create --name param-create-invalid --param missing=value - <<<"$(param_template)"
    assert_fatal 'error while processing flag "param": template does not define param "missing"'
}

@test 'edit accepts --param shortcut' {
    run -0 limactl create --name param-edit - <<<"$(param_template)"

    run -0 limactl edit --param release=v1.36 param-edit

    run -0 limactl yq -r .param.release <"${LIMA_HOME}/param-edit/lima.yaml"
    assert_output "v1.36"
}
