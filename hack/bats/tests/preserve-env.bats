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
    export LIMA_SHELLENV_ALLOW="FOO*, , BAR"
    export FOO=foo
    export FOOBAR=foobar
    export BAR=bar
    export BARBAZ=barbaz
    run -0 limactl shell --preserve-env "$NAME" printenv
    assert_line FOO=foo
    assert_line FOOBAR=foobar
    assert_line BAR=bar
    assert_line BARBAZ=barbaz
}

@test 'wildcard patterns work in all positions and combinations' {
    # Test wildcard at middle, and other combinations
    export LIMA_SHELLENV_BLOCK="FOO*BAR,*FOO*BAR*,*TEST*,*SUFFIX"
    export FOO=foo
    export FOOBAR=foobar
    export FOOXYZBAR=fooxyzbar
    export FOOBAZ=foobaz
    export BAZBAR=bazbar
    export BAR=bar
    export XFOOYBARZDOTCOM=xfooybarzdotcom
    export PREFIX_TEST_VAR=prefix_test_var
    export VAR_SUFFIX=var_suffix
    export NORMAL_VAR=normal_var
    export UNRELATED=unrelated
    run -0 limactl shell --preserve-env "$NAME" printenv
    
    # Should block FOO*BAR pattern
    refute_line --regexp '^FOOBAR='
    refute_line --regexp '^FOOXYZBAR='
    
    # Should block *FOO*BAR* pattern
    refute_line --regexp '^XFOOYBARZDOTCOM='
    
    # Should block *TEST* and *SUFFIX patterns
    refute_line --regexp '^PREFIX_TEST_VAR='
    refute_line --regexp '^VAR_SUFFIX='
    
    # Should allow variables that don't match any pattern
    assert_line FOO=foo
    assert_line FOOBAZ=foobaz
    assert_line BAZBAR=bazbar
    assert_line BAR=bar
    assert_line NORMAL_VAR=normal_var
    assert_line UNRELATED=unrelated
}

@test 'comprehensive allow/block interaction with wildcards and default blocklist' {
    # Test allowlist with wildcards, and other test rules
    export LIMA_SHELLENV_ALLOW="SSH_FOO,CUSTOM*,FOO*,*PREFIX,MIDDLE*PATTERN,SUFFIX*"
    export LIMA_SHELLENV_BLOCK="+*TOKEN"
    export SSH_FOO=ssh_foo
    export SSH_BAR=ssh_bar
    export CUSTOM_VAR=custom_var
    export MY_TOKEN=my_token
    export SECRET_TOKEN=secret_token
    export FOO=foo
    export FOOBAR=foobar
    export BAR=bar
    export BARBAZ=barbaz
    export TEST_PREFIX=test_prefix
    export MIDDLE_TEST_PATTERN=middle_test_pattern
    export SUFFIX_TEST=suffix_test
    export OTHER_VAR=other_var
    export NORMAL_VAR=normal_var
    run -0 limactl shell --preserve-env "$NAME" printenv
    
    # Should allow items in allowlist even if they match default blocklist
    assert_line SSH_FOO=ssh_foo
    assert_line CUSTOM_VAR=custom_var
    assert_line FOO=foo
    assert_line FOOBAR=foobar
    assert_line TEST_PREFIX=test_prefix
    assert_line MIDDLE_TEST_PATTERN=middle_test_pattern
    assert_line SUFFIX_TEST=suffix_test
    
    # Should block SSH_BAR (default blocklist, not in allowlist)
    refute_line --regexp '^SSH_BAR='
    
    # Should block *TOKEN (additive pattern)
    refute_line --regexp '^MY_TOKEN='
    refute_line --regexp '^SECRET_TOKEN='
    
    # Should allow other variables not in blocklist
    assert_line BAR=bar
    assert_line BARBAZ=barbaz
    assert_line OTHER_VAR=other_var
    assert_line NORMAL_VAR=normal_var
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
