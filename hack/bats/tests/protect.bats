# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

INSTANCE=bats-dummy
CLONE=clone
NOTEXIST=notexist

local_setup_file() {
    local inst
    for inst in "$CLONE" "$NOTEXIST"; do
        limactl unprotect "$inst" || :
        limactl delete --force "$inst" || :
    done
}

@test 'protecting a non-existing instance fails' {
    run_e -1 limactl protect "${NOTEXIST}"
    assert_fatal "failed to inspect instance \"${NOTEXIST}\"…"
}

@test 'protecting the dummy instance succeeds' {
    run_e -0 limactl protect "$INSTANCE"
    assert_info "Protected \"${INSTANCE}\""
    assert_file_exists "${LIMA_HOME}/${INSTANCE}/protected"
}

@test 'protecting it again shows a warning, but succeeds' {
    run_e -0 limactl protect "$INSTANCE"
    assert_warning "Instance \"${INSTANCE}\" is already protected. Skipping."
    assert_file_exists "${LIMA_HOME}/${INSTANCE}/protected"
}

@test 'cloning a protected instance creates an unprotected clone' {
    run_e -0 limactl clone --yes "$INSTANCE" "$CLONE"
    # TODO there is currently no output from the clone command, which feels wrong
    refute_output
    assert_file_not_exists "${LIMA_HOME}/${CLONE}/protected"
}

@test 'deleting the unprotected clone instance succeeds' {
    run_e -0 limactl delete --force "$CLONE"
    assert_info "Deleted \"${CLONE}\"…"
}

@test 'deleting protected dummy instance fails' {
    run_e -1 limactl delete --force "$INSTANCE"
    assert_fatal "failed to delete instance \"${INSTANCE}\": instance is protected…"
    assert_file_exists "$LIMA_HOME/$INSTANCE/protected"
}

@test 'unprotecting the dummy instance succeeds' {
    run_e -0 limactl unprotect "$INSTANCE"
    assert_info "Unprotected \"${INSTANCE}\""
    assert_file_not_exists "$LIMA_HOME/$INSTANCE/protected"
}

@test 'unprotecting it again shows a warning, but succeeds' {
    run_e -0 limactl unprotect "$INSTANCE"
    assert_warning "Instance \"${INSTANCE}\" isn't protected. Skipping."
    assert_file_not_exists "$LIMA_HOME/$INSTANCE/protected"
}

@test 'deleting unprotected dummy instance succeeds' {
    run_e -0 limactl delete --force "$INSTANCE"
    assert_info "Deleted \"${INSTANCE}\"…"
}
