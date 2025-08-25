#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"

if [ "$#" -ne 1 ]; then
	ERROR "Usage: $0 NAME"
	exit 1
fi

NAME="$1"

INFO "Testing --preserve-env flag"

# Environment variable propagation with --preserve-env
INFO "=== Environment variable propagation with --preserve-env ==="
test_var_name="LIMA_TEST_PRESERVE_ENV_VAR"
test_var_value="test-value-${RANDOM}"

got="$(LIMA_SHELLENV_ALLOW="LIMA_TEST_*" LIMA_TEST_PRESERVE_ENV_VAR="$test_var_value" limactl shell --preserve-env "$NAME" printenv "$test_var_name" 2>/dev/null || echo "NOT_FOUND")"
INFO "$test_var_name: expected=${test_var_value}, got=${got}"
if [ "$got" != "$test_var_value" ]; then
	ERROR "Environment variable was not propagated with --preserve-env"
	exit 1
fi

# Clean up before next test
unset LIMA_TEST_PRESERVE_ENV_VAR
unset LIMA_SHELLENV_ALLOW

# Environment variable NOT propagated without --preserve-env
INFO "=== Environment variable NOT propagated without --preserve-env ==="
got_without_flag="$(LIMA_TEST_PRESERVE_ENV_VAR="$test_var_value" limactl shell "$NAME" printenv "$test_var_name" 2>/dev/null || echo "NOT_FOUND")"
INFO "$test_var_name without --preserve-env: got=${got_without_flag}"
if [ "$got_without_flag" != "NOT_FOUND" ]; then
	ERROR "Environment variable was unexpectedly propagated without --preserve-env"
	exit 1
fi

# Blocked environment variables should not be propagated even with --preserve-env
INFO "=== Blocked environment variables are not propagated with --preserve-env ==="
blocked_var_name="HOME"
fake_home="/tmp/fake-home-${RANDOM}"
got_blocked="$(HOME="$fake_home" limactl shell --preserve-env "$NAME" printenv "$blocked_var_name" 2>/dev/null || echo "NOT_FOUND")"
INFO "$blocked_var_name: host=${fake_home}, guest=${got_blocked}"
if [ "$got_blocked" = "$fake_home" ]; then
	ERROR "Blocked environment variable $blocked_var_name was propagated"
	exit 1
fi

# LIMA_SHELLENV_BLOCK functionality
INFO "=== LIMA_SHELLENV_BLOCK with custom pattern ==="
custom_test_var="LIMA_TEST_CUSTOM_BLOCK"
custom_test_value="should-be-blocked"
got_custom_blocked="$(LIMA_SHELLENV_BLOCK="+LIMA_TEST_CUSTOM_*" LIMA_TEST_CUSTOM_BLOCK="$custom_test_value" limactl shell --preserve-env "$NAME" printenv "$custom_test_var" 2>/dev/null || echo "NOT_FOUND")"
INFO "$custom_test_var with LIMA_SHELLENV_BLOCK: got=${got_custom_blocked}"
if [ "$got_custom_blocked" != "NOT_FOUND" ]; then
	ERROR "Custom blocked environment variable was propagated"
	exit 1
fi

# Clean up before next test
unset LIMA_TEST_CUSTOM_BLOCK 2>/dev/null || true
unset LIMA_SHELLENV_BLOCK 2>/dev/null || true

# LIMA_SHELLENV_ALLOW functionality
INFO "=== LIMA_SHELLENV_ALLOW with custom pattern ==="
allow_test_var="LIMA_TEST_ALLOW_VAR"
allow_test_value="should-be-allowed"
got_allowed="$(LIMA_SHELLENV_ALLOW="LIMA_TEST_ALLOW_*" LIMA_TEST_ALLOW_VAR="$allow_test_value" limactl shell --preserve-env "$NAME" printenv "$allow_test_var" 2>/dev/null || echo "NOT_FOUND")"
INFO "$allow_test_var with LIMA_SHELLENV_ALLOW: got=${got_allowed}"
if [ "$got_allowed" != "$allow_test_value" ]; then
	ERROR "Allowed environment variable was not propagated"
	exit 1
fi

# Non-allowed variables are blocked when LIMA_SHELLENV_ALLOW is set
INFO "=== Non-allowed variables are blocked when LIMA_SHELLENV_ALLOW is set ==="
other_test_var="LIMA_TEST_OTHER_VAR"
other_test_value="should-be-blocked"
got_other="$(LIMA_SHELLENV_ALLOW="LIMA_TEST_ALLOW_*" LIMA_TEST_OTHER_VAR="$other_test_value" limactl shell --preserve-env "$NAME" printenv "$other_test_var" 2>/dev/null || echo "NOT_FOUND")"
INFO "$other_test_var with LIMA_SHELLENV_ALLOW (should be blocked): got=${got_other}"
if [ "$got_other" != "NOT_FOUND" ]; then
	ERROR "Non-allowed environment variable was propagated when LIMA_SHELLENV_ALLOW was set"
	exit 1
fi

INFO "All --preserve-env tests passed"
