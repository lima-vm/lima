#!/bin/bash
set -euxo pipefail

INSTANCE=plain-static-port-forward
TEMPLATE=hack/test-templates/plain-static-port-forward.yaml

limactl delete -f $INSTANCE || true

limactl start --name=$INSTANCE --tty=false $TEMPLATE

limactl shell $INSTANCE -- bash -c 'until [ -e /run/nginx.pid ]; do sleep 1; done'

curl -sSf http://127.0.0.1:9090 | grep -i 'nginx' && echo 'nginx port forwarding works!'

if curl -sSf http://127.0.0.1:9080; then
  echo 'ERROR: Port 9080 should not be forwarded in plain mode!'
  exit 1
else
  echo 'Port 9080 is correctly NOT forwarded in plain mode.'
fi

limactl delete -f $INSTANCE 