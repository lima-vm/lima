#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -euxo pipefail

INSTANCE=plain-static-port-forward
TEMPLATE=hack/test-templates/static-port-forward.yaml

limactl delete -f $INSTANCE || true

limactl start --name=$INSTANCE --plain=true --tty=false $TEMPLATE

limactl shell $INSTANCE -- bash -c 'until [ -e /run/nginx.pid ]; do sleep 1; done'

curl -sSf http://127.0.0.1:9090 | grep -i 'nginx' && echo 'Static port forwarding (9090) works in plain mode!'

if curl -sSf http://127.0.0.1:9080 2>/dev/null; then
	echo 'ERROR: Dynamic port 9080 should not be forwarded in plain mode!'
	exit 1
else
	echo 'Dynamic port 9080 is correctly NOT forwarded in plain mode.'
fi

if curl -sSf http://127.0.0.1:9070 2>/dev/null; then
	echo 'ERROR: Dynamic port 9070 should not be forwarded in plain mode!'
	exit 1
else
	echo 'Dynamic port 9070 is correctly NOT forwarded in plain mode.'
fi

limactl delete -f $INSTANCE
echo "All tests passed for plain mode - only static ports work!"
# EOF
