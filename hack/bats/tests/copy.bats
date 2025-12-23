# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=bats

# TODO The reusable Lima instance setup is copied from preserve-env.bats
# TODO and should be factored out into helper functions.
local_setup_file() {
    if [[ -n "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        run limactl list --format '{{.Status}}' "$NAME"
        [[ $status == 0 ]] && [[ $output == "Running" ]] && return
    fi
    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
    # Make sure that the host agent doesn't inherit file handles 3 or 4.
    # Otherwise bats will not finish until the host agent exits.
    limactl start --yes --name "$NAME" template:default 3>&- 4>&-
}

local_teardown_file() {
    if [[ -z "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        limactl delete --force "$NAME"
    fi
}

setup() {
    limactl shell "$NAME" -- mkdir -p /tmp/test_limactl_copy
}

teardown() {
    limactl shell "$NAME" -- rm -rf /tmp/test_limactl_copy
}

test_copy_dir_from_host_to_instance() {
    backend="$1"
    mkdir -p "$BATS_TEST_TMPDIR/foo/bar"

    # SRC DST
    run limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo" "$NAME":/tmp/test_limactl_copy/foo
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- test -d /tmp/test_limactl_copy/foo/bar
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- rm -rf /tmp/test_limactl_copy/foo
    [[ $status == 0 ]]

    # SRC/ DST
    run limactl shell "$NAME" -- mkdir -p /tmp/test_limactl_copy/foo_src_with_slash/
    [[ $status == 0 ]]
    run limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo/" "$NAME":/tmp/test_limactl_copy/foo_src_with_slash
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- test -d /tmp/test_limactl_copy/foo_src_with_slash/foo
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- rm -rf /tmp/test_limactl_copy/foo_src_with_slash
    [[ $status == 0 ]]

    # SRC DST/
    run limactl shell "$NAME" -- mkdir -p /tmp/test_limactl_copy/foo_dst_with_slash/
    [[ $status == 0 ]]
    run limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo" "$NAME":/tmp/test_limactl_copy/foo_dst_with_slash/
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- test -d /tmp/test_limactl_copy/foo_dst_with_slash/foo
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- rm -rf /tmp/test_limactl_copy/foo_dst_with_slash
    [[ $status == 0 ]]

    # SRC/ DST/
    run limactl shell "$NAME" -- mkdir -p /tmp/test_limactl_copy/foo_src_dst_with_slash/
    [[ $status == 0 ]]
    run limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo/" "$NAME":/tmp/test_limactl_copy/foo_src_dst_with_slash/
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- test -d /tmp/test_limactl_copy/foo_src_dst_with_slash/foo
    [[ $status == 0 ]]
    run limactl shell "$NAME" -- rm -rf /tmp/test_limactl_copy/foo_src_dst_with_slash
    [[ $status == 0 ]]
}

@test "copy directory from host to Lima instance (scp)" {
    test_copy_dir_from_host_to_instance scp
}

# https://github.com/lima-vm/lima/issues/4468
@test "copy directory from host to Lima instance (rsync)" {
    test_copy_dir_from_host_to_instance rsync
}

test_copy_dir_from_instance_to_host() {
    backend="$1"

    run limactl shell "$NAME" -- mkdir -p /tmp/test_limactl_copy/foo/bar
    [[ $status == 0 ]]

    # SRC DST
    run limactl copy --backend="$backend" -r "$NAME":/tmp/test_limactl_copy/foo "$BATS_TEST_TMPDIR/foo"
    [[ $status == 0 ]]
    [[ -d "$BATS_TEST_TMPDIR/foo/bar" ]]

    # SRC/ DST
    run limactl copy --backend="$backend" -r "$NAME":/tmp/test_limactl_copy/foo/ "$BATS_TEST_TMPDIR/foo_src_with_slash"
    [[ $status == 0 ]]
    [[ -d "$BATS_TEST_TMPDIR/foo_src_with_slash/bar" ]]

    # SRC DST/
    run limactl copy --backend="$backend" -r "$NAME":/tmp/test_limactl_copy/foo "$BATS_TEST_TMPDIR/foo_dst_with_slash/"
    [[ $status == 0 ]]
    [[ -d "$BATS_TEST_TMPDIR/foo_dst_with_slash/bar" ]]

    # SRC/ DST/
    run limactl copy --backend="$backend" -r "$NAME":/tmp/test_limactl_copy/foo/ "$BATS_TEST_TMPDIR/foo_src_dst_with_slash/"
    [[ $status == 0 ]]
    [[ -d "$BATS_TEST_TMPDIR/foo_src_dst_with_slash/bar" ]]
}

@test "copy directory from Lima instance to host (scp)" {
    test_copy_dir_from_instance_to_host scp
}

@test "copy directory from Lima instance to host (rsync)" {
    test_copy_dir_from_instance_to_host rsync
}

test_copy_file_from_host_to_instance() {
    backend="$1"
    echo "hello" > "$BATS_TEST_TMPDIR/hello.txt"

    run limactl copy --backend="$backend" "$BATS_TEST_TMPDIR/hello.txt" "$NAME":/tmp/test_limactl_copy/hello.txt
    [[ $status == 0 ]]

    run limactl shell "$NAME" -- cat /tmp/test_limactl_copy/hello.txt
    [[ $status == 0 ]]
    [[ $output == "hello" ]]

    run limactl shell "$NAME" -- rm -f /tmp/test_limactl_copy/hello.txt
    [[ $status == 0 ]]
}

@test "copy file from host to Lima instance (scp)" {
    test_copy_file_from_host_to_instance scp
}

@test "copy file from host to Lima instance (rsync)" {
    test_copy_file_from_host_to_instance rsync
}

test_copy_file_from_instance_to_host() {
    backend="$1"
    run limactl shell "$NAME" -- bash -c 'echo "hello" > /tmp/test_limactl_copy/hello.txt'
    [[ $status == 0 ]]

    run limactl copy --backend="$backend" "$NAME":/tmp/test_limactl_copy/hello.txt "$BATS_TEST_TMPDIR/hello.txt"
    [[ $status == 0 ]]

    run cat "$BATS_TEST_TMPDIR/hello.txt"
    [[ $status == 0 ]]
    [[ $output == "hello" ]]

    run limactl shell "$NAME" -- rm -f /tmp/hello.txt
    [[ $status == 0 ]]
}

@test "copy file from Lima instance to host (scp)" {
    test_copy_file_from_instance_to_host scp
}

@test "copy file from Lima instance to host (rsync)" {
    test_copy_file_from_instance_to_host rsync
}
