#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -euxo pipefail

INSTANCE=nonplain-static-port-forward
TEMPLATE="$(dirname "$0")/test-templates/test-misc.yaml"

limactl delete -f "$INSTANCE" || true

limactl start --name="$INSTANCE" --tty=false "$TEMPLATE"

limactl shell "$INSTANCE" -- bash -c 'until systemctl is-active --quiet nginx; do sleep 1; done'
limactl shell "$INSTANCE" -- bash -c 'until systemctl is-active --quiet test-server-9080; do sleep 1; done'
limactl shell "$INSTANCE" -- bash -c 'until systemctl is-active --quiet test-server-9070; do sleep 1; done'

curl -sSf http://127.0.0.1:9090 | grep -i 'nginx' && echo 'Static port forwarding (9090) works in normal mode!'
curl -sSf http://127.0.0.1:29080 | grep -i 'Dynamic port 9080' && echo 'Dynamic port forwarding (29080) works in normal mode!'
curl -sSf http://127.0.0.1:29070 | grep -i 'Dynamic port 9070' && echo 'Dynamic port forwarding (29070) works in normal mode!'

limactl delete -f "$INSTANCE"
echo "All tests passed for normal mode - both static and dynamic ports work!"
# EOF
