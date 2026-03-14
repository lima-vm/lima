# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o nounset -o pipefail

# Make sure run() will execute all functions with errexit enabled.
# This enables the functionality from https://github.com/lima-vm/bats-core/commit/507da84de75fa78798d53eceb42e68851ef5c48b
# The upstream PR https://github.com/bats-core/bats-core/pull/1118 is still open, so our submodule points to the PR commit.
export BATS_RUN_ERREXIT=1

# BATS_TEST_RETRIES must be set for the individual test and cannot be imported from the
# parent environment because the BATS test runner sets it to 0 before running the test.
BATS_TEST_RETRIES=${LIMA_BATS_ALL_TESTS_RETRIES:-0}

# Known flaky tests should call `flaky` inside the @test to allow retries up to
# LIMA_BATS_FLAKY_TESTS_RETRIES even when the LIMA_BATS_ALL_TESTS_RETRIES is lower.
flaky() {
    BATS_TEST_RETRIES=${LIMA_BATS_FLAKY_TESTS_RETRIES:-$BATS_TEST_RETRIES}
}

# Don't run the tests in ~/.lima because they may destroy _config, _templates etc.
export LIMA_HOME=${LIMA_BATS_LIMA_HOME:-$HOME/.lima-bats}

absolute_path() {
    (
        cd "$1"
        pwd
    )
}

PATH_BATS_HELPERS=$(absolute_path "$(dirname "${BASH_SOURCE[0]}")")
PATH_BATS_ROOT=$(absolute_path "$PATH_BATS_HELPERS/..")

source "$PATH_BATS_ROOT/lib/bats-support/load.bash"
source "$PATH_BATS_ROOT/lib/bats-assert/load.bash"
source "$PATH_BATS_ROOT/lib/bats-file/load.bash"

source "$PATH_BATS_HELPERS/limactl.bash"
source "$PATH_BATS_HELPERS/logs.bash"

bats_require_minimum_version 1.5.0

run_e() {
    run --separate-stderr "$@"
}

# If called from foo() this function will call local_foo() if it exist.
call_local_function() {
    local func
    func="local_${FUNCNAME[1]}"
    if [ "$(type -t "$func")" = "function" ]; then
        "$func"
    fi
}

setup_file() {
    if [[ ${CI:-} == true ]]; then
        # Without a terminal the output is using TAP formatting, which does not include the filename
        local TEST_FILENAME=${BATS_TEST_FILENAME#"$PATH_BATS_ROOT/tests/"}
        TEST_FILENAME=${TEST_FILENAME%.bats}
        echo "# ===== ${TEST_FILENAME} =====" >&3
    fi
    if [[ -n "${INSTANCE:-}" ]]; then
        ensure_instance "$INSTANCE"
    fi
    call_local_function
}
teardown_file() {
    call_local_function
    if [[ -n "${INSTANCE:-}" ]]; then
        delete_instance "$INSTANCE"
    fi
}
setup() {
    call_local_function
}
teardown() {
    call_local_function
}

assert_output_lines_count() {
    assert_equal "${#lines[@]}" "$1"
}

# Use GHCR and ECR to avoid hitting Docker Hub rate limit.
# NOTE: keep this list in sync with hack/test-templates.sh .
declare -A -g TEST_CONTAINER_IMAGES=(
    ["nginx"]="ghcr.io/stargz-containers/nginx:1.19-alpine-org"
    ["coredns"]="public.ecr.aws/eks-distro/coredns/coredns:v1.12.2-eks-1-31-latest"
)
