# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

INSTANCE=bats-nomount

setup() {
    # Create a temporary test directory and files
    TEST_SYNC_DIR="$BATS_TEST_TMPDIR/sync-test"
    mkdir -p "$TEST_SYNC_DIR"
    touch "$TEST_SYNC_DIR/foo.txt"
    touch "$TEST_SYNC_DIR/bar.txt"

    # Create a simple script that makes changes to these files
    cat > "$TEST_SYNC_DIR/modify.sh" << 'EOF'
#!/bin/sh
set -eu
echo "modified foo" > foo.txt
echo "modified bar" > bar.txt
EOF
    chmod +x "$TEST_SYNC_DIR/modify.sh"
}

teardown() {
    # Clean up test directory
    if [[ -d "$TEST_SYNC_DIR" ]]; then
        rm -rf "$TEST_SYNC_DIR"
    fi
}

@test 'shell --sync preserves working directory path from host to guest' {
    cd "$TEST_SYNC_DIR"

    # Get path of the TEST_SYNC_DIR for verification
    local path_test_dir
    path_test_dir="$PWD"

    run -0 bash -c "limactl shell --sync . --yes '$INSTANCE' pwd && ./modify.sh"

    # Verify the guest working directory matches the host path structure
    assert_output --regexp ".*${path_test_dir#/}"

    # Verify files were modified
    run cat "$TEST_SYNC_DIR/foo.txt"
    assert_output "modified foo"
    run cat "$TEST_SYNC_DIR/bar.txt"
    assert_output "modified bar"
}

@test 'shell --sync with directory path containing spaces and quotes' {
    # Create directory with spaces and quotes in name and move test directory to it
    local special_dir="$BATS_TEST_TMPDIR/sync test 'with' \"quotes\""
    mkdir -p "$special_dir"
    mv "$TEST_SYNC_DIR" "$special_dir"
    cd "$special_dir/sync-test"

    # Count files before sync
    local files_before
    files_before=$(find . -type f | wc -l)

    run -0 bash -c "limactl shell --sync . --yes '$INSTANCE' ./modify.sh"

    # Verify files were modified
    run cat "$special_dir/sync-test/foo.txt"
    assert_output "modified foo"
    run cat "$special_dir/sync-test/bar.txt"
    assert_output "modified bar"

    # Count files after sync
    local files_after
    files_after=$(find "$special_dir/sync-test" -type f | wc -l)
    [[ $files_after -eq $files_before ]]

    # Cleanup
    rm -rf "$special_dir"
}

@test 'shell --sync reflects file deletion from guest to host' {
    cd "$TEST_SYNC_DIR"

    run -0 bash -c "limactl shell --sync . --yes '$INSTANCE' rm -f foo.txt"

    assert_file_not_exists "$TEST_SYNC_DIR/foo.txt"
}

@test 'shell --sync reflects new directory and file creation from guest to host' {
    cd "$TEST_SYNC_DIR"

    # Create a script that creates a new directory with a file
    cat > "$TEST_SYNC_DIR/create_new.sh" << 'EOF'
#!/bin/sh
set -eu
mkdir -p new_directory
echo "foo bar baz" > new_directory/new_file.txt
EOF
    chmod +x "$TEST_SYNC_DIR/create_new.sh"

    run -0 bash -c "limactl shell --sync . --yes '$INSTANCE' ./create_new.sh && ./modify.sh"

    # Verify new directory was created on host
    assert_dir_exists "$TEST_SYNC_DIR/new_directory"
    assert_file_exists "$TEST_SYNC_DIR/new_directory/new_file.txt"

    # Verify file content
    run cat "$TEST_SYNC_DIR/new_directory/new_file.txt"
    assert_output "foo bar baz"
    run cat "$TEST_SYNC_DIR/foo.txt"
    assert_output "modified foo"
    run cat "$TEST_SYNC_DIR/bar.txt"
    assert_output "modified bar"
}

@test 'shell --sync preserves file permissions' {
    cd "$TEST_SYNC_DIR"

    # Create a file with specific permissions
    touch "$TEST_SYNC_DIR/executable.sh"
    chmod 755 "$TEST_SYNC_DIR/executable.sh"

    # Modify the file in guest
    run -0 bash -c "limactl shell --sync . --yes '$INSTANCE' ./modify.sh"

    # Verify file is still executable on host
    if [[ "$OSTYPE" == darwin* ]]; then
        run stat -f '%A' "$TEST_SYNC_DIR/executable.sh"
    else
        run stat -c '%a' "$TEST_SYNC_DIR/executable.sh"
    fi
    assert_output "755"

    # Verify files were modified
    run cat "$TEST_SYNC_DIR/foo.txt"
    assert_output "modified foo"
    run cat "$TEST_SYNC_DIR/bar.txt"
    assert_output "modified bar"
}

@test 'shell --sync works without existing ControlMaster socket' {
    cd "$TEST_SYNC_DIR"

    # Remove the ControlMaster socket
    local sock_path="$LIMA_HOME/$INSTANCE/ssh.sock"
    if [[ -S "$sock_path" ]]; then
        rm "$sock_path"
    fi

    run -0 bash -c "limactl shell --sync . --yes '$INSTANCE' ./modify.sh"

    # Verify files were modified
    run cat "$TEST_SYNC_DIR/foo.txt"
    assert_output "modified foo"
    run cat "$TEST_SYNC_DIR/bar.txt"
    assert_output "modified bar"
}
