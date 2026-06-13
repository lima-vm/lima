# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

INSTANCE=bats

setup() {
    limactl shell "$INSTANCE" -- mkdir -p /tmp/test_limactl_copy
}

teardown() {
    limactl shell "$INSTANCE" -- rm -rf /tmp/test_limactl_copy
}

test_copy_dir_from_host_to_instance() {
    backend="$1"
    mkdir -p "$BATS_TEST_TMPDIR/foo/bar"

    # SRC DST
    limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo" "$INSTANCE":/tmp/test_limactl_copy/foo
    limactl shell "$INSTANCE" -- test -d /tmp/test_limactl_copy/foo/bar
    limactl shell "$INSTANCE" -- rm -rf /tmp/test_limactl_copy/foo

    # SRC/ DST
    limactl shell "$INSTANCE" -- mkdir -p /tmp/test_limactl_copy/foo_src_with_slash/
    limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo/" "$INSTANCE":/tmp/test_limactl_copy/foo_src_with_slash
    limactl shell "$INSTANCE" -- test -d /tmp/test_limactl_copy/foo_src_with_slash/foo
    limactl shell "$INSTANCE" -- rm -rf /tmp/test_limactl_copy/foo_src_with_slash

    # SRC DST/
    limactl shell "$INSTANCE" -- mkdir -p /tmp/test_limactl_copy/foo_dst_with_slash/
    limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo" "$INSTANCE":/tmp/test_limactl_copy/foo_dst_with_slash/
    limactl shell "$INSTANCE" -- test -d /tmp/test_limactl_copy/foo_dst_with_slash/foo
    limactl shell "$INSTANCE" -- rm -rf /tmp/test_limactl_copy/foo_dst_with_slash

    # SRC/ DST/
    limactl shell "$INSTANCE" -- mkdir -p /tmp/test_limactl_copy/foo_src_dst_with_slash/
    limactl copy --backend="$backend" -r "$BATS_TEST_TMPDIR/foo/" "$INSTANCE":/tmp/test_limactl_copy/foo_src_dst_with_slash/
    limactl shell "$INSTANCE" -- test -d /tmp/test_limactl_copy/foo_src_dst_with_slash/foo
    limactl shell "$INSTANCE" -- rm -rf /tmp/test_limactl_copy/foo_src_dst_with_slash
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

    limactl shell "$INSTANCE" -- mkdir -p /tmp/test_limactl_copy/foo/bar

    # SRC DST
    limactl copy --backend="$backend" -r "$INSTANCE":/tmp/test_limactl_copy/foo "$BATS_TEST_TMPDIR/foo"
    assert_dir_exists "$BATS_TEST_TMPDIR/foo/bar"

    # SRC/ DST
    limactl copy --backend="$backend" -r "$INSTANCE":/tmp/test_limactl_copy/foo/ "$BATS_TEST_TMPDIR/foo_src_with_slash"
    assert_dir_exists "$BATS_TEST_TMPDIR/foo_src_with_slash/bar"

    # SRC DST/
    limactl copy --backend="$backend" -r "$INSTANCE":/tmp/test_limactl_copy/foo "$BATS_TEST_TMPDIR/foo_dst_with_slash/"
    assert_dir_exists "$BATS_TEST_TMPDIR/foo_dst_with_slash/bar"

    # SRC/ DST/
    limactl copy --backend="$backend" -r "$INSTANCE":/tmp/test_limactl_copy/foo/ "$BATS_TEST_TMPDIR/foo_src_dst_with_slash/"
    assert_dir_exists "$BATS_TEST_TMPDIR/foo_src_dst_with_slash/bar"
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

    limactl copy --backend="$backend" "$BATS_TEST_TMPDIR/hello.txt" "$INSTANCE":/tmp/test_limactl_copy/hello.txt

    run -0 limactl shell "$INSTANCE" -- cat /tmp/test_limactl_copy/hello.txt
    assert_output "hello"

    limactl shell "$INSTANCE" -- rm -f /tmp/test_limactl_copy/hello.txt
}

@test "copy file from host to Lima instance (scp)" {
    test_copy_file_from_host_to_instance scp
}

@test "copy file from host to Lima instance (rsync)" {
    test_copy_file_from_host_to_instance rsync
}

test_copy_file_from_instance_to_host() {
    backend="$1"
    limactl shell "$INSTANCE" -- bash -c 'echo "hello" > /tmp/test_limactl_copy/hello.txt'

    limactl copy --backend="$backend" "$INSTANCE":/tmp/test_limactl_copy/hello.txt "$BATS_TEST_TMPDIR/hello.txt"

    run -0 cat "$BATS_TEST_TMPDIR/hello.txt"
    assert_output "hello"

    limactl shell "$INSTANCE" -- rm -f /tmp/hello.txt
}

@test "copy file from Lima instance to host (scp)" {
    test_copy_file_from_instance_to_host scp
}

@test "copy file from Lima instance to host (rsync)" {
    test_copy_file_from_instance_to_host rsync
}
