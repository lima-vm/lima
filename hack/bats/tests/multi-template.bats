# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=multi-template

local_setup_file() {
    limactl delete --force "$NAME" || :
}

local_teardown_file() {
    limactl delete --force "$NAME" || :
}

local_teardown() {
    limactl delete --force "$NAME" || :
}

@test 'create with multiple templates merges values' {
    # Create base template file (uses /etc/profile as dummy image like create_dummy_instance)
    cat > "${BATS_TEST_TMPDIR}/base.yaml" <<'EOF'
images:
- location: /etc/profile
cpus: 3
EOF

    # Create override file with additional settings
    cat > "${BATS_TEST_TMPDIR}/override.yaml" <<'EOF'
memory: 5GiB
disk: 37GiB
EOF

    run -0 limactl create --name "$NAME" "${BATS_TEST_TMPDIR}/base.yaml" "${BATS_TEST_TMPDIR}/override.yaml"

    # Verify the base values were used (cpus should be 3 from first template)
    run -0 limactl list --format '{{.CPUs}}' "$NAME"
    assert_output "3"

    # Verify the override values were merged (memory should be from second template)
    # 5GiB = 5368709120 bytes
    run -0 limactl list --format '{{.Memory}}' "$NAME"
    assert_output "5368709120"
}

@test 'first template values take precedence for scalars' {
    cat > "${BATS_TEST_TMPDIR}/base.yaml" <<'EOF'
images:
- location: /etc/profile
cpus: 3
memory: 3GiB
EOF

    cat > "${BATS_TEST_TMPDIR}/override.yaml" <<'EOF'
cpus: 7
memory: 7GiB
EOF

    run -0 limactl create --name "$NAME" "${BATS_TEST_TMPDIR}/base.yaml" "${BATS_TEST_TMPDIR}/override.yaml"

    # cpus from first template should win
    run -0 limactl list --format '{{.CPUs}}' "$NAME"
    assert_output "3"

    # memory from first template should win
    # 3GiB = 3221225472 bytes
    run -0 limactl list --format '{{.Memory}}' "$NAME"
    assert_output "3221225472"
}

@test 'relative paths resolve from current directory' {
    mkdir -p "${BATS_TEST_TMPDIR}/testdir"

    cat > "${BATS_TEST_TMPDIR}/testdir/base.yaml" <<'EOF'
images:
- location: /etc/profile
cpus: 5
EOF

    cat > "${BATS_TEST_TMPDIR}/testdir/config.yaml" <<'EOF'
memory: 7GiB
EOF

    cd "${BATS_TEST_TMPDIR}/testdir"
    run -0 limactl create --name "$NAME" base.yaml config.yaml

    run -0 limactl list --format '{{.CPUs}}' "$NAME"
    assert_output "5"

    # 7GiB = 7516192768 bytes
    run -0 limactl list --format '{{.Memory}}' "$NAME"
    assert_output "7516192768"
}

@test 'multiple args with existing instance produces error' {
    # Create an instance first using stdin
    limactl create --name "$NAME" - <<'EOF'
images:
- location: /etc/profile
EOF

    cat > "${BATS_TEST_TMPDIR}/extra.yaml" <<'EOF'
cpus: 3
EOF

    # Attempting to start the existing instance with additional templates should error
    run ! limactl start "$NAME" "${BATS_TEST_TMPDIR}/extra.yaml"
    assert_output --partial "cannot specify additional templates"
}

@test 'instance name derived from first template filename' {
    # Clean up any stale myinstance from previous runs
    limactl delete --force myinstance || :

    cat > "${BATS_TEST_TMPDIR}/myinstance.yaml" <<'EOF'
images:
- location: /etc/profile
EOF

    cat > "${BATS_TEST_TMPDIR}/override.yaml" <<'EOF'
cpus: 3
EOF

    run -0 limactl create "${BATS_TEST_TMPDIR}/myinstance.yaml" "${BATS_TEST_TMPDIR}/override.yaml"

    # Instance should be named after first template
    run -0 limactl list --format '{{.Name}}'
    assert_output --partial "myinstance"

    limactl delete --force myinstance || :
}

@test 'explicit --name flag overrides derived name' {
    cat > "${BATS_TEST_TMPDIR}/base.yaml" <<'EOF'
images:
- location: /etc/profile
EOF

    cat > "${BATS_TEST_TMPDIR}/override.yaml" <<'EOF'
cpus: 5
EOF

    run -0 limactl create --name "$NAME" "${BATS_TEST_TMPDIR}/base.yaml" "${BATS_TEST_TMPDIR}/override.yaml"

    run -0 limactl list --format '{{.Name}}' "$NAME"
    assert_output "$NAME"
}

@test 'three templates merge correctly' {
    cat > "${BATS_TEST_TMPDIR}/t1.yaml" <<'EOF'
images:
- location: /etc/profile
cpus: 3
EOF

    cat > "${BATS_TEST_TMPDIR}/t2.yaml" <<'EOF'
memory: 5GiB
EOF

    cat > "${BATS_TEST_TMPDIR}/t3.yaml" <<'EOF'
disk: 73GiB
EOF

    run -0 limactl create --name "$NAME" "${BATS_TEST_TMPDIR}/t1.yaml" "${BATS_TEST_TMPDIR}/t2.yaml" "${BATS_TEST_TMPDIR}/t3.yaml"

    run -0 limactl list --format '{{.CPUs}}' "$NAME"
    assert_output "3"

    # 5GiB = 5368709120 bytes
    run -0 limactl list --format '{{.Memory}}' "$NAME"
    assert_output "5368709120"

    # 73GiB = 78383153152 bytes
    run -0 limactl list --format '{{.Disk}}' "$NAME"
    assert_output "78383153152"
}

@test 'stdin can be used as one of the templates' {
    cat > "${BATS_TEST_TMPDIR}/override.yaml" <<'EOF'
memory: 5GiB
EOF

    # Use stdin as the first template, with a file as the second
    run -0 limactl create --name "$NAME" - "${BATS_TEST_TMPDIR}/override.yaml" <<'EOF'
images:
- location: /etc/profile
cpus: 7
EOF

    # cpus from stdin template should be used
    run -0 limactl list --format '{{.CPUs}}' "$NAME"
    assert_output "7"

    # memory from override file should be merged
    # 5GiB = 5368709120 bytes
    run -0 limactl list --format '{{.Memory}}' "$NAME"
    assert_output "5368709120"
}
