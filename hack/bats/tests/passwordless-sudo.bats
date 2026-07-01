# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"
CUSTOM_INSTANCE="test-passwordlessSudo"
local_setup_file() {
	limactl delete -f "${CUSTOM_INSTANCE}" 2>/dev/null || true
	limactl start --name="${CUSTOM_INSTANCE}" --tty=false \
		--set '.user.passwordlessSudo = false' \
		--plain \
		template:default
}

local_teardown_file() {
	limactl delete -f "${CUSTOM_INSTANCE}"
}

@test "passwordlessSudo: false - sudo without password fails" {
	run -1 limactl shell "${CUSTOM_INSTANCE}" -- sudo -n true
}

@test "passwordlessSudo: false - password file exists with correct permissions" {
	limactl shell "${CUSTOM_INSTANCE}" -- bash -c 'stat ~/password'
	run -0 limactl shell "${CUSTOM_INSTANCE}" -- bash -c 'stat -c "%a" ~/password'
	assert_output "600"
}

@test "passwordlessSudo: false - generated password authenticates sudo" {
	limactl shell "${CUSTOM_INSTANCE}" -- bash -c 'echo "$(cat ~/password)" | sudo -S true'
}

@test "passwordlessSudo: false - password file is not regenerated on reboot" {
	original=$(limactl shell "${CUSTOM_INSTANCE}" -- bash -c 'cat ~/password')

	limactl stop "${CUSTOM_INSTANCE}"
	limactl start --tty=false "${CUSTOM_INSTANCE}"

	run -0 limactl shell "${CUSTOM_INSTANCE}" -- bash -c 'cat ~/password'
	assert_output "${original}"
}
