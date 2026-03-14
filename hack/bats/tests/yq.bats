# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

@test 'make sure the yq subcommand exists' {
    run -0 limactl yq --version
    assert_output --regexp '^yq .*mikefarah.* version v'
}

@test 'yq can evaluate yq expressions' {
    run -0 limactl yq -n .foo=42
    assert_output 'foo: 42'
}

@test 'yq command understand yq options' {
    run -0 limactl yq -n -o json -I 0 .foo=42
    assert_output '{"foo":42}'
}

@test 'yq errors set non-zero exit code' {
    run -1 limactl yq -n foo
    assert_output --partial "invalid input"
}

@test 'yq works as a multi-call binary' {
    # multi-call command detection strips all extensions
    YQ="yq.lima.exe"
    ln -sf "$(which limactl)" "${BATS_TEST_TMPDIR}/${YQ}"
    export PATH="$BATS_TEST_TMPDIR:$PATH"

    run -0 "$YQ" --version
    assert_output --regexp '^yq .*mikefarah.* version v'

    run -0 "$YQ" -n -o json -I 0 .foo=42
    assert_output '{"foo":42}'
}

@test 'yq multi-call command has support for env access' {
    export FOO=bar
    run -0 limactl yq -n 'env(FOO)'
    assert_output "bar"
}

@test 'yq multi-call command has support for --security-disable-env-ops' {
    export FOO=bar
    run_e -1 limactl yq -n --security-disable-env-ops 'env(FOO)'
    assert_stderr "Error: env operations have been disabled"
}
