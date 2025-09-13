#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -euxo pipefail

INSTANCE=plain-static-port-forward
TEMPLATE="$(dirname "$0")/test-templates/test-misc.yaml"

limactl delete -f "$INSTANCE" || true

limactl start --name="$INSTANCE" --plain=true --tty=false "$TEMPLATE"

limactl shell "$INSTANCE" -- bash -c 'until systemctl is-active --quiet nginx; do sleep 1; done'

if ! curl -sSf http://127.0.0.1:9090 | grep -i 'nginx'; then
	echo 'ERROR: Static port forwarding (9090) does not work in plain mode!'
	exit 1
fi
echo 'Static port forwarding (9090) works in plain mode!'

if curl -sSf http://127.0.0.1:29080 2>/dev/null; then
	echo 'ERROR: Dynamic port 29080 should not be forwarded in plain mode!'
	exit 1
else
	echo 'Dynamic port 29080 is correctly NOT forwarded in plain mode.'
fi

if curl -sSf http://127.0.0.1:29070 2>/dev/null; then
	echo 'ERROR: Dynamic port 29070 should not be forwarded in plain mode!'
	exit 1
else
	echo 'Dynamic port 29070 is correctly NOT forwarded in plain mode.'
fi

limactl delete -f "$INSTANCE"
echo "All tests passed for plain mode - only static ports work!"
# EOF
