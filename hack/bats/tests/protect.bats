# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=dummy
CLONE=clone
NOTEXIST=notexist

local_setup_file() {
    for INSTANCE in "$NAME" "$CLONE" "$NOTEXIST"; do
        limactl unprotect "$INSTANCE" || :
        limactl delete --force "$INSTANCE" || :
    done
}

@test 'create dummy instance' {
    run -0 create_dummy_instance "$NAME" '.disk = "1M"'
}

@test 'protecting a non-existing instance fails' {
    run_e -1 limactl protect "${NOTEXIST}"
    assert_fatal "failed to inspect instance \"${NOTEXIST}\"…"
}

@test 'protecting the dummy instance succeeds' {
    run_e -0 limactl protect "$NAME"
    assert_info "Protected \"${NAME}\""
    assert_file_exists "${LIMA_HOME}/${NAME}/protected"
}

@test 'protecting it again shows a warning, but succeeds' {
    run_e -0 limactl protect "$NAME"
    assert_warning "Instance \"${NAME}\" is already protected. Skipping."
    assert_file_exists "${LIMA_HOME}/${NAME}/protected"
}

@test 'cloning a protected instance creates an unprotected clone' {
    run_e -0 limactl clone --yes "$NAME" "$CLONE"
    # TODO there is currently no output from the clone command, which feels wrong
    refute_output
    assert_file_not_exists "${LIMA_HOME}/${CLONE}/protected"
}

@test 'deleting the unprotected clone instance succeeds' {
    run_e -0 limactl delete --force "$CLONE"
    assert_info "Deleted \"${CLONE}\"…"
}

@test 'deleting protected dummy instance fails' {
    run_e -1 limactl delete --force "$NAME"
    assert_fatal "failed to delete instance \"${NAME}\": instance is protected…"
    assert_file_exists "$LIMA_HOME/$NAME/protected"
}

@test 'unprotecting the dummy instance succeeds' {
    run_e -0 limactl unprotect "$NAME"
    assert_info "Unprotected \"${NAME}\""
    assert_file_not_exists "$LIMA_HOME/$NAME/protected"
}

@test 'unprotecting it again shows a warning, but succeeds' {
    run_e -0 limactl unprotect "$NAME"
    assert_warning "Instance \"${NAME}\" isn't protected. Skipping."
    assert_file_not_exists "$LIMA_HOME/$NAME/protected"
}

@test 'deleting unprotected dummy instance succeeds' {
    run_e -0 limactl delete --force "$NAME"
    assert_info "Deleted \"${NAME}\"…"
}
