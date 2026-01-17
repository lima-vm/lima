# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

NAME=bats

# TODO Move helper functions to shared location
run_yq() {
    run -0 --separate-stderr limactl yq "$@"
}

json_edit() {
    limactl yq --input-format json --output-format json --indent 0 "$@"
}

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

local_setup() {
    cd "$PATH_BATS_ROOT"
    coproc MCP { limactl mcp serve "$NAME"; }

    ID=0
    mcp initialize '{"protocolVersion":"2025-06-18"}'

    # Each mcp request should increment the ID
    [[ $ID -eq 1 ]]

    run_yq .serverInfo.name <<<"$output"
    assert_output "lima"
}

local_teardown() {
    kill "${MCP_PID:?}" 2>&1 >/dev/null || :
}

mcp() {
    local method=$1
    local params=${2:-}

    local request
    printf -v request '{"jsonrpc":"2.0","id":%d,"method":"%s"}' "$((++ID))" "$method"
    if [[ -n $params ]]; then
        request=$(json_edit ".params=${params}" <<<"$request")
    fi

    # send request to MCP server stdin
    echo "$request" >&"${MCP[1]}"

    # read response from MCP server stdout with 5s timeout
    local json
    while true; do
        if ! read -t 5 -r json <&"${MCP[0]}"; then
            break
        fi
        # If it has no "method" field, it's a response, not a notification
        if ! jq -e 'has("method")' <<<"$json" >/dev/null 2>&1; then
            break
        fi
    done

    # verify that the response matches the request; also validates the output is valid JSON
    run_yq .id <<<"$json"
    assert_output "$ID"

    # there must be no error object in the response
    run_yq .error <<<"$json"
    assert_output "null"

    # set $output to .result
    run_yq .result <<<"$json"
}

tools_call() {
    local name=$1
    local args=${2:-}

    local params
    printf -v params '{"name":"%s"}' "$name"
    if [[ -n $args ]]; then
        params=$(json_edit ".arguments=${args}" <<<"$params")
    fi
    mcp tools/call "$params"
}

@test 'list tools' {
    mcp tools/list
    run_yq '.tools[].name' <<<"$output"
    assert_line glob
    assert_line list_directory
    assert_line read_file
    assert_line run_shell_command
    assert_line search_file_content
    assert_line write_file
}

@test 'verify that tools descriptions include input and output schema' {
    mcp tools/list
    run_yq '.tools[] | select(.name == "run_shell_command")' <<<"$output"
    json=$output

    run_yq '.inputSchema.required[]' <<<"$json"
    assert_line command
    assert_line directory
    assert_output_lines_count 2

    run_yq '.inputSchema.properties | keys[]' <<<"$json"
    assert_line command
    assert_line description
    assert_line directory
    assert_output_lines_count 3

    run_yq '.outputSchema.required[]' <<<"$json"
    assert_line stdout
    assert_line stderr
    assert_output_lines_count 2

    run_yq '.outputSchema.properties | keys[]' <<<"$json"
    assert_line error
    assert_line exit_code
    assert_line stdout
    assert_line stderr
    assert_output_lines_count 4
}

@test 'run shell command returns command output' {
    run -0 limactl shell "$NAME" cat /etc/os-release
    assert_output
    expected=$output

    tools_call run_shell_command '{"directory":"/etc","command":["cat","os-release"]}'
    json=$output

    run_yq '.structuredContent.exit_code' <<<"$json"
    assert_output 0

    run_yq '.structuredContent.stdout' <<<"$json"
    assert_output "$expected"

    run_yq '.structuredContent.stderr' <<<"$json"
    refute_output

    # The same data is also available as encoded JSON
    run_yq '.content[0].type' <<<"$json"
    assert_output "text"

    run_yq '.content[0].text' <<<"$json"
    text=$output

    run_yq '.exit_code' <<<"$text"
    assert_output 0

    run_yq '.stdout' <<<"$text"
    assert_output "$expected"

    run_yq '.stderr' <<<"$text"
    refute_output
}

@test 'run shell command returns stderr and exit code' {
    tools_call run_shell_command '{"directory":"/","command":["bash","-c","echo NO>&2; exit 13"]}'
    json=$output

    run_yq '.structuredContent.exit_code' <<<"$json"
    assert_output 13

    run_yq '.structuredContent.error' <<<"$json"
    assert_output "exit status 13"

    run_yq '.structuredContent.stdout' <<<"$json"
    refute_output

    run_yq '.structuredContent.stderr' <<<"$json"
    assert_output "NO"
}

@test 'run shell command fails if the directory does not exist' {
    tools_call run_shell_command '{"directory":"/etcetera","command":["cat","os-release"]}'
    json=$output

    run_yq '.structuredContent.exit_code' <<<"$json"
    assert_output 1

    run_yq '.structuredContent.stderr' <<<"$json"
    assert_output --partial "No such file or directory"
}

@test 'read_file reads a file' {
    run -0 limactl shell "$NAME" cat /etc/os-release
    assert_output
    expected=$output

    tools_call read_file '{"path":"/etc/os-release"}'
    json=$output

    run_yq '.content[0].text' <<<"$json"
    run_yq '.content' <<<"$output"
    assert_output "$expected"

    run_yq '.structuredContent.content' <<<"$json"
    assert_output "$expected"
}

@test 'read_file returns an error when path does not exist' {
    tools_call read_file '{"path":"/etc/os-release-info"}'
    json=$output

    run_yq '.isError' <<<"$json"
    assert_output "true"

    run_yq '.content[0].text' <<<"$json"
    assert_output "file does not exist"
}

@test 'read_file returns an error when path is not absolute' {
    tools_call read_file '{"path":"os-release"}'
    json=$output

    run_yq '.isError' <<<"$json"
    assert_output "true"

    run_yq '.content[0].text' <<<"$json"
    assert_output --partial "expected an absolute path"
}

@test 'write_file creates new file and overwrites existing file' {
    limactl shell "$NAME" rm -f /tmp/mcp.test
    tools_call write_file '{"path":"/tmp/mcp.test","content":"foo"}'

    run_yq '.content[0].text' <<<"$output"
    assert_output "{}"

    run -0 limactl shell "$NAME" cat /tmp/mcp.test
    assert_output "foo"

    tools_call write_file '{"path":"/tmp/mcp.test","content":"bar"}'

    run_yq '.content[0].text' <<<"$output"
    assert_output "{}"

    run -0 limactl shell "$NAME" cat /tmp/mcp.test
    assert_output "bar"
}

@test 'write_file creates the directory if it does not yet exist' {
    # Make sure /tmp/tmp is deletable even if we run the tests multiple times against the same Lima instance
    limactl shell "$NAME" chmod -R 777 /tmp/tmp || true
    limactl shell "$NAME" rm -rf /tmp/tmp
    tools_call write_file '{"path":"/tmp/tmp/tmp","content":"tmp"}'
    json=$output

    run_yq '.isError' <<<"$json"
    assert_output "null"

    run -0 limactl shell "$NAME" cat /tmp/tmp/tmp
    assert_output "tmp"
}

@test 'write_file returns an error when the directory is not writable' {
    limactl shell "$NAME" mkdir -p /tmp/tmp
    limactl shell "$NAME" chmod 444 /tmp/tmp
    tools_call write_file '{"path":"/tmp/tmp/tmp","content":"tmp"}'
    json=$output

    run_yq '.isError' <<<"$json"
    assert_output "true"

    run_yq '.content[0].text' <<<"$json"
    assert_output "permission denied"
}

@test 'write_file returns an error when path is not absolute' {
    tools_call write_file '{"path":"tmp/mcp.test","content":"baz"}'
    json=$output

    run_yq '.isError' <<<"$json"
    assert_output "true"

    run_yq '.content[0].text' <<<"$json"
    assert_output --partial "expected an absolute path"
}

@test 'glob finds files by wildcard' {
    tools_call glob '{"pattern":"*/*p.bats"}'

    run_yq '.structuredContent.matches[]' <<<"$output"
    assert_line --regexp '/tests/mcp.bats$'
}

@test 'glob returns an empty list when the pattern does not match' {
   
    tools_call glob '{"pattern":"nothing.to.see"}'

    run_yq '.structuredContent.matches[]' <<<"$output"
    assert_output_lines_count 0
}

@test 'search_file_content finds text inside files' {
    tools_call search_file_content '{"pattern":"needle in a haystack"}'

    run_yq '.structuredContent.git_grep_output' <<<"$output"
    assert_line --regexp '^tests/mcp.bats:[0-9]+: +tools_call'
}

@test 'search_file_content can find unicode characters above U+FFFF' {
    # The light bulb emoji 💡 (U+1F4A1)
    tools_call search_file_content '{"pattern":"💡"}'

    run_yq '.structuredContent.git_grep_output' <<<"$output"
    assert_line --regexp '^tests/mcp.bats:[0-9]+: +# The light bulb'
    assert_line --regexp '^tests/mcp.bats:[0-9]+: +tools_call'
}

@test 'search_file_content returns an empty string if it cannot find the pattern' {
    tools_call search_file_content "$(printf '{"pattern":"\U0001f4a1 not found"}')"

    run_yq '.structuredContent.git_grep_output' <<<"$output"
    refute_output
}

