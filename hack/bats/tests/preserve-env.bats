# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

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

@test 'wildcard works at the start of the pattern' {
    export LIMA_SHELLENV_BLOCK="*FOO"
    export FOO=foo
    export BARFOO=barfoo
    run -0 limactl shell --preserve-env "$NAME" printenv
    refute_line --regexp '^BARFOO='
    refute_line --regexp '^FOO='
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

@test 'allow list can use a , separated list with whitespace ignored' {
    export LIMA_SHELLENV_ALLOW="SSH_FOO, , BAR*, LD_UID"
    export SSH_FOO=ssh_foo
    export SSH_BAR=ssh_bar
    export SSH_BLOCK=ssh_block
    export BAR=bar
    export BARBAZ=barbaz
    export LD_UID=randomuid
    run -0 limactl shell --preserve-env "$NAME" printenv
    
    assert_line SSH_FOO=ssh_foo
    assert_line BAR=bar
    assert_line BARBAZ=barbaz
    assert_line LD_UID=randomuid
    
    refute_line --regexp '^SSH_BAR='
    refute_line --regexp '^SSH_BLOCK='
}

@test 'wildcard patterns work in all positions' {
    export LIMA_SHELLENV_BLOCK="*FOO*BAR*"
    export FOO=foo
    export FOOBAR=foobar
    export FOOXYZBAR=fooxyzbar
    export FOOBAZ=foobaz
    export BAZBAR=bazbar
    export BAR=bar
    export XFOOYBARZDOTCOM=xfooybarzdotcom
    export NORMAL_VAR=normal_var
    export UNRELATED=unrelated
    run -0 limactl shell --preserve-env "$NAME" printenv
    
    refute_line --regexp '^FOOBAR='
    refute_line --regexp '^FOOXYZBAR='    
    refute_line --regexp '^XFOOYBARZDOTCOM='
    
    assert_line FOOBAZ=foobaz
    assert_line NORMAL_VAR=normal_var
    assert_line UNRELATED=unrelated
    assert_line BAZBAR=bazbar
    assert_line BAR=bar
    assert_line FOO=foo
}

@test 'allowlist overrides default blocklist with wildcards' {
    export LIMA_SHELLENV_ALLOW="SSH_*,CUSTOM*"
    export LIMA_SHELLENV_BLOCK="+*TOKEN"
    export SSH_AUTH_SOCK=ssh_auth_sock
    export SSH_CONNECTION=ssh_connection
    export CUSTOM_VAR=custom_var
    export MY_TOKEN=my_token
    export UNRELATED=unrelated
    run -0 limactl shell --preserve-env "$NAME" printenv
    
    assert_line SSH_AUTH_SOCK=ssh_auth_sock
    assert_line SSH_CONNECTION=ssh_connection
    assert_line CUSTOM_VAR=custom_var
    refute_line --regexp '^MY_TOKEN='
    assert_line UNRELATED=unrelated
}

@test 'invalid characters in patterns cause fatal errors' {
    export LIMA_SHELLENV_BLOCK="FOO-BAR"
    run ! limactl shell --preserve-env "$NAME" printenv
    assert_output --partial "Invalid LIMA_SHELLENV_BLOCK pattern"
    assert_output --partial "contains invalid character"
}

@test 'limactl info includes the default block list' {
    run -0 limactl info
    run -0 limactl yq '.shellEnvBlock[]' <<<"$output"
    assert_line PATH
    assert_line "SSH_*"
    assert_line USER
}
