#!/bin/bash
set -euxo pipefail

INSTANCE=nonplain-static-port-forward
TEMPLATE=hack/test-templates/nonplain-static-port-forward.yaml

limactl delete -f $INSTANCE || true

limactl start --name=$INSTANCE --tty=false $TEMPLATE

limactl shell $INSTANCE -- bash -c 'until [ -e /run/nginx.pid ]; do sleep 1; done'

curl -sSf http://127.0.0.1:9090 | grep -i 'nginx' && echo 'nginx port forwarding works!'

limactl delete -f $INSTANCE 