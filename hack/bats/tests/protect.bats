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
    # needs an image that clonefile() can process; /dev/null doesn't work
    limactl create --name "$NAME" - <<<"{images: [location: /etc/profile], disk: 1M}"
}

@test 'protecting a non-existing instance fails' {
    run -1 limactl protect "${NOTEXIST}"
    assert_output --partial 'failed to inspect instance'
}

@test 'protecting the dummy instance succeeds' {
    run -0 limactl protect "$NAME"
    assert_output --regexp "Protected \\\\\"$NAME\\\\\""
    assert_file_exists "$LIMA_HOME/$NAME/protected"
}

@test 'protecting it again shows a warning, but succeeds' {
    run -0 limactl protect "$NAME"
    assert_output --partial 'already protected. Skipping'
    assert_file_exists "$LIMA_HOME/$NAME/protected"
}

@test 'cloning a protected instance creates an unprotected clone' {
    run -0 limactl clone --yes "$NAME" "$CLONE"
    # TODO there is currently no output from the clone command, which feels wrong
    refute_output
    assert_file_not_exists "$LIMA_HOME/$CLONE/protected"
}

@test 'deleting the unprotected clone instance succeeds' {
    run -0 limactl delete --force "$CLONE"
    assert_output --regexp "Deleted \\\\\"$CLONE\\\\\""
}

@test 'deleting protected dummy instance fails' {
    run -1 limactl delete --force "$NAME"
    assert_output --partial 'instance is protected'
    assert_file_exists "$LIMA_HOME/$NAME/protected"
}

@test 'unprotecting the dummy instance succeeds' {
    run -0 limactl unprotect "$NAME"
    assert_output --regexp "Unprotected \\\\\"$NAME\\\\\""
    assert_file_not_exists "$LIMA_HOME/$NAME/protected"
}

@test 'unprotecting it again shows a warning, but succeeds' {
    run -0 limactl unprotect "$NAME"
    assert_output --partial "isn't protected. Skipping"
    assert_file_not_exists "$LIMA_HOME/$NAME/protected"
}

@test 'deleting unprotected dummy instance succeeds' {
    run -0 limactl delete --force "$NAME"
    assert_output --regexp "Deleted \\\\\"$NAME\\\\\""
}
