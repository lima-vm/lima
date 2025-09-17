# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

# Use a separate LIMA_HOME for this test that we can just wipe because it will never have running instances.
# We cannot use $BATS_FILE_TMPDIR because `limactl create` will complain about max socket path name length.
LOCAL_LIMA_HOME="${LIMA_HOME:?}/_bats"

local_setup_file() {
    export LIMA_HOME="${LOCAL_LIMA_HOME:?}"
    rm -rf "${LIMA_HOME:?}"

    run -0 create_dummy_instance "foo" '.disk = "1M"'
    run -0 create_dummy_instance "bar" '.disk = "2M"'
    run -0 create_dummy_instance "baz" '.disk = "3M"'
}

local_setup() {
    export LIMA_HOME="${LOCAL_LIMA_HOME:?}"
}

@test 'list with no running instances shows a warning and exits without error' {
    export LIMA_HOME="$BATS_TEST_TMPDIR"
    run_e -0 limactl list
    assert_warning 'No instance found. Run `limactl create` to create an instance.'
}

@test 'check plain list output' {
    # also verifies that `ls` is an alias for `list`
    run -0 limactl ls

    # instances will be sorted alphabetically
    assert_line --index 0 --regexp '^NAME +STATUS.+DISK'
    assert_line --index 1 --regexp '^bar +Stopped.+ 2MiB'
    assert_line --index 2 --regexp '^baz +Stopped.+ 3MiB'
    assert_line --index 3 --regexp '^foo +Stopped.+ 1MiB'
    # there is no other output
    assert_output_lines_count 4
}

@test 'only list selected instances' {
    run -0 limactl ls foo bar

    # instances will be sorted in the order they were specified
    assert_line --index 0 --regexp '^NAME'
    assert_line --index 1 --regexp '^foo'
    assert_line --index 2 --regexp '^bar'
    refute_line --partial baz
    assert_output_lines_count 3
}

@test 'requesting non-existing instance is an error' {
    run_e -1 limactl ls foo foobar bar
    assert_warning 'No instance matching foobar found.'
    assert_fatal 'unmatched instances'

    # existing instances are still listed
    assert_line --index 0 --regexp '^NAME'
    assert_line --index 1 --regexp '^foo'
    assert_line --index 2 --regexp '^bar'
    assert_output_lines_count 3
}

@test '--quiet option shows only names, no header' {
    run -0 limactl list --quiet foo bar
    assert_line --index 0 foo
    assert_line --index 1 bar
    assert_output_lines_count 2
}

@test '--format json returns JSON output' {
    run -0 limactl ls --format json foo bar

    # test may be too strict in expecting "name" to be the first key on the line
    assert_line --index 0 --regexp '^\{"name":"foo",'
    assert_line --index 1 --regexp '^\{"name":"bar",'
    assert_output_lines_count 2
}

@test '--json is shorthand for --format json' {
    run -0 limactl ls --format json foo bar
    format_json=$output

    run -0 limactl ls --json foo bar
    assert_output "$format_json"
}

@test '--format YAML returns YAML documents' {
    # save JSON output for comparison
    run -0 limactl ls foo bar --format json
    json=$output

    run -0 limactl ls --format yaml foo bar
    yaml=$output

    assert_line --regexp '^name: foo'
    assert_line --regexp '^name: bar'
    refute_line --regexp '^name: baz'

    # verify that the output consists of 2 documents
    run -0 limactl yq 'true' <<<"$yaml"
    assert_output_lines_count 2

    # convert YAML to JSON
    run -0 limactl yq --input-format yaml --output-format json  --indent 0 "." <<<"$yaml"
    assert_output_lines_count 2

    # verify it matches the JSON output
    assert_output "$json"

}

@test 'JSON output to terminal is colorized, but semantically identical' {
    run -0 limactl ls foo bar --format json
    json=$output

    # colorize output even when stdout is not a tty
    export _LIMA_OUTPUT_IS_TTY=1
    run -0 limactl ls foo bar --format json
    colorized=$output

    # check if the output contains an ANSI "reset mode" sequence ("ESC[0m")
    run -0 cat -v <<<"$output"
    assert_output --partial "^[[0m"

    # remove all ANSI formatting codes from the output
    run -0 sed 's/\x1b\[[0-9;]*m//g' <<<"$colorized"

    # Flatten the pretty-printed JSON to a single line per object with no extra whitespace.
    # yq processes JSON objects in sequence (when input format is set to json) and
    # does not require each object to be on a single line.
    run -0 limactl yq --input-format json --output-format json  --indent 0 "." <<<"$output"
    assert_output_lines_count 2

    # compare to the plain (uncolorized) json output
    assert_output "$json"
}

@test 'YAML output to terminal is colorized, but semantically identical' {
    # save uncolorized JSON output
    run -0 limactl ls foo bar --format json
    json=$output

    # colorize output even when stdout is not a tty
    export _LIMA_OUTPUT_IS_TTY=1
    run -0 limactl ls foo bar --format yaml
    colorized=$output

    # check if the output contains an ANSI "reset mode" sequence ("ESC[0m")
    run -0 cat -v <<<"$output"
    assert_output --partial "^[[0m"

    # remove all ANSI formatting codes from the output
    run -0 sed 's/\x1b\[[0-9;]*m//g' <<<"$colorized"
    yaml=$output

    # Verify that the output consists of 2 documents
    run -0 limactl yq 'true' <<<"$yaml"
    assert_output_lines_count 2

    # convert the pretty-printed YAML to JSON Lines format with no whitespace
    run -0 limactl yq --indent 0 --input-format yaml --output-format json "." <<<"$yaml"
    assert_output_lines_count 2

    # verify it matches the JSON output
    assert_output "$json"
}

@test '--all-fields includes all fields in the table' {
    skip "only works with output to a terminal (#3986)"
    # TODO provide a way to specify the width, e.g. with `--width 120`
    # See https://github.com/lima-vm/lima/issues/3986
}

@test 'Use field names in Go template format' {
    run -0 limactl ls foo bar --format '{{.Name}} {{.Disk}}'
    assert_line --index 0 "foo 1048576"
    assert_line --index 1 "bar 2097152"
}

@test '--list-fields list all available fields' {
    run -0 limactl ls --list-fields
    assert_line Name
    assert_line CPUs
    assert_line Memory
    # All field names start with an uppercase letter and don't contain any spaces
    refute_line --regexp '^[^A-Z]'
    refute_line --partial ' '
 }

 @test '--list-fields does not list deprecated field, but they are still available' {
    run -0 limactl ls --list-fields

    # no deprecated fields are listed
    refute_line "HostArch"
    refute_line "HostOS"
    refute_line "IdentityFile"
    refute_line "LimaHome"

    # all deprecated fields exist and produce output
    run -0 limactl ls foo --format '{{.HostArch}}'
    assert_output
    run -0 limactl ls foo --format '{{.HostOS}}'
    assert_output
    run -0 limactl ls foo --format '{{.IdentityFile}}'
    assert_output
    run -0 limactl ls foo --format '{{.LimaHome}}'
    assert_output "$LIMA_HOME"

    # verify that a non-existing field throws an error and produces no output
    # TODO the error message is not really end-user friendly, not sure if we can do something about it
    run_e -1 limactl ls foo --format '{{.Unknown}}'
    assert_stderr --regexp "level=fatal.*can't evaluate field Unknown"
    refute_output
}

@test '--quiet option can only be used with format --table' {
    # TODO the error message is incorrect, it can be used with --yq
    run_e -1 limactl list --quiet --format json
    assert_fatal "option --quiet can only be used with '--format table'"

    run_e -1 limactl list --quiet --format yaml
    assert_fatal "option --quiet can only be used with '--format table'"

    run_e -1 limactl list --quiet --format '{{.Name}} {{.Disk}}'
    assert_fatal "option --quiet can only be used with '--format table'"
}

@test '--yq option implies --format json' {
    run -0 limactl ls baz --yq '.config'
    assert_output --regexp '^\{"'

    run -0 limactl yq '.disk' <<<"$output"
    assert_output "3M"
}

@test '--yq option can be used with --format yaml' {
    run -0 limactl ls baz --yq '.config' --format yaml
    assert_line "disk: 3M"
}

@test '--yq option can be specified multiple times' {
    run -0 limactl ls foo --yq '.config' --yq '.user' --yq '.uid'
    assert_output "$UID"

    run -0 limactl ls --yq 'select(.disk > 1024*1024)' --yq 'select(.name | test("z"))' --yq '.name'
    assert_output "baz"
}

@test '--yq option is incompatible with --format table or Go templates' {
    run_e -1 limactl ls --yq '.name' --format table
    assert_fatal "option --yq only works with --format json or yaml"

    run_e -1 limactl ls --yq '.name' --format '{{.Name}} {{.Disk}}'
    assert_fatal "option --yq only works with --format json or yaml"
}

@test '--quiet option can be used with --yq' {
    run -0 limactl ls --quiet
    assert_line --index 0 "bar"
    assert_output_lines_count 3

    run -0 limactl ls --quiet --yq 'select(.name == "foo")'
    assert_output "foo"
}
