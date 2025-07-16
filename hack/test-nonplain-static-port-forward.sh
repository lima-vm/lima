#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -euxo pipefail

INSTANCE=nonplain-static-port-forward
TEMPLATE=hack/test-templates/static-port-forward.yaml

limactl delete -f $INSTANCE || true

limactl start --name=$INSTANCE --tty=false $TEMPLATE

limactl shell $INSTANCE -- bash -c 'until [ -e /run/nginx.pid ]; do sleep 1; done'
limactl shell $INSTANCE -- bash -c 'until systemctl is-active --quiet test-server-9080; do sleep 1; done'
limactl shell $INSTANCE -- bash -c 'until systemctl is-active --quiet test-server-9070; do sleep 1; done'

curl -sSf http://127.0.0.1:9090 | grep -i 'nginx' && echo 'Static port forwarding (9090) works in normal mode!'
curl -sSf http://127.0.0.1:9080 | grep -i 'Dynamic port 9080' && echo 'Dynamic port forwarding (9080) works in normal mode!'
curl -sSf http://127.0.0.1:9070 | grep -i 'Dynamic port 9070' && echo 'Dynamic port forwarding (9070) works in normal mode!'

limactl delete -f $INSTANCE
echo "All tests passed for normal mode - both static and dynamic ports work!"
# EOF
