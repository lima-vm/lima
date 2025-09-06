load "../helpers/load"

NAME=bats

local_setup_file() {
    unset LIMA_SHELLENV_ALLOW
    unset LIMA_SHELLENV_BLOCK

    if [[ -n "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        run limactl list --format '{{.Status}}' "$NAME"
        [[ $status == 0 ]] && [[ $output == "Running" ]] && return
    fi
    limactl unprotect "$NAME" || :
    limactl delete --force "$NAME" || :
    # Make sure that the host agent doesn't inherit file handles 3 or 4.
    # Otherwise bats will not finish until the host agent exits.
    limactl start --yes --name "$NAME" template://default 3>&- 4>&-
}

local_teardown_file() {
    if [[ -z "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        limactl delete --force "$NAME"
    fi
}

local_setup() {
    # make sure changes from previous tests are removed
    limactl shell "$NAME" sh -c '[ ! -f ~/.bash_profile ] || sed -i -E "/^export (FOO|BAR|SSH_)/d" ~/.bash_profile'
}

@test 'there are no FOO*, BAR*, or SSH_FOO* variables defined in the VM' {
    # just to confirm because the other tests depend on these being unused
    run -0 limactl shell "$NAME" printenv
    refute_line --regexp '^FOO'
    refute_line --regexp '^BAR'
    refute_line --regexp '^SSH_FOO'
}

@test 'environment is not preserved by default' {
    export FOO=foo
    run -0 limactl shell "$NAME" printenv
    refute_line --regexp '^FOO='
}

@test 'environment is preserved with --preserve-env' {
    export FOO=foo
    run -0 limactl shell --preserve-env "$NAME" printenv
    assert_line FOO=foo
}

@test 'profile settings inside the VM take precedence over preserved variables' {
    limactl shell "$NAME" sh -c 'echo "export FOO=bar" >>~/.bash_profile'
    export FOO=foo
    run -0 limactl shell --preserve-env "$NAME" printenv
    assert_line FOO=bar
}

@test 'builtin block list is used when LIMA_SHELLENV_BLOCK is not set' {
    # default block list includes SSH_*
    export SSH_FOO=ssh_foo
    run -0 limactl shell --preserve-env "$NAME" printenv
    refute_line --regexp '^SSH_FOO='
}

@test 'custom block list replaces builtin block list' {
    export LIMA_SHELLENV_BLOCK=FOO
    export FOO=foo
    export SSH_FOO=foo
    run -0 limactl shell --preserve-env "$NAME" printenv
    refute_line --regexp '^FOO='
    assert_line SSH_FOO=foo
}

@test 'custom block list starting with + appends to builtin block list' {
    export LIMA_SHELLENV_BLOCK=+FOO
    export FOO=foo
    export SSH_FOO=foo
    run -0 limactl shell --preserve-env "$NAME" printenv
    refute_line --regexp '^FOO='
    refute_line --regexp '^SSH_FOO='
}

@test 'block list entries can use * wildcard at the end' {
    export LIMA_SHELLENV_BLOCK="FOO*"
    export FOO=foo
    export FOOBAR=foobar
    export BAR=bar
    run -0 limactl shell --preserve-env "$NAME" printenv
    refute_line --regexp '^FOO'
    assert_line BAR=bar
}

@test 'wildcard does only work at the end of the pattern' {
    export LIMA_SHELLENV_BLOCK="*FOO"
    export FOO=foo
    export BARFOO=barfoo
    run -0 limactl shell --preserve-env "$NAME" printenv
    assert_line FOO=foo
    assert_line BARFOO=barfoo
}

@test 'block list can use a , separated list with whitespace ignored' {
    export LIMA_SHELLENV_BLOCK="FOO*, , BAR"
    export FOO=foo
    export FOOBAR=foobar
    export BAR=bar
    export BARBAZ=barbaz
    run -0 limactl shell --preserve-env "$NAME" printenv
    refute_line --regexp '^FOO'
    refute_line --regexp '^BAR='
    assert_line BARBAZ=barbaz
}

@test 'allow list overrides block list but blocks everything else' {
    export LIMA_SHELLENV_ALLOW=SSH_FOO
    export SSH_FOO=ssh_foo
    export SSH_BAR=ssh_bar
    export BAR=bar
    run -0 limactl shell --preserve-env "$NAME" printenv
    assert_line SSH_FOO=ssh_foo
    refute_line --regexp '^SSH_BAR='
    refute_line --regexp '^BAR='
}

@test 'allow list can use a , separated list with whitespace ignored' {
    export LIMA_SHELLENV_ALLOW="FOO*, , BAR"
    export FOO=foo
    export FOOBAR=foobar
    export BAR=bar
    export BARBAZ=barbaz
    run -0 limactl shell --preserve-env "$NAME" printenv
    assert_line FOO=foo
    assert_line FOOBAR=foobar
    assert_line BAR=bar
    refute_line --regexp '^BARBAZ='
}

@test 'setting both allow list and block list generates a warning' {
    export LIMA_SHELLENV_ALLOW=FOO
    export LIMA_SHELLENV_BLOCK=BAR
    export FOO=foo
    run -0 --separate-stderr limactl shell --preserve-env "$NAME" printenv FOO
    assert_output foo
    assert_stderr --regexp 'level=warning msg="Both LIMA_SHELLENV_BLOCK and LIMA_SHELLENV_ALLOW are set'
}

@test 'limactl info includes the default block list' {
    run -0 limactl info
    run -0 limactl yq '.shellEnvBlock[]' <<<"$output"
    assert_line PATH
    assert_line "SSH_*"
    assert_line USER
}
