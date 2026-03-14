# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Format the string the way strconv.Quote() would do.
# If the input ends with an ellipsis then no closing quote will be added (and the … will be removed).
quote_msg() {
    local quoted
    quoted=$(sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' -e 's/^/"/' <<<"$1")
    if [[ $quoted == *… ]]; then
        echo "${quoted%…}"
    else
        echo "${quoted}\""
  fi
}

assert_fatal() {
    assert_stderr_line --partial "level=fatal msg=$(quote_msg "$1")"
}
assert_error() {
    assert_stderr_line --partial "level=error msg=$(quote_msg "$1")"
}
assert_warning() {
    assert_stderr_line --partial "level=warning msg=$(quote_msg "$1")"
}
assert_info() {
    assert_stderr_line --partial "level=info msg=$(quote_msg "$1")"
}
assert_debug() {
    assert_stderr_line --partial "level=debug msg=$(quote_msg "$1")"
}
