# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o nounset -o pipefail

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
    call_local_function
}
teardown_file() {
    call_local_function
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
