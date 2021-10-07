#!/bin/bash
set -eux -o pipefail

sed -i '/host.lima.internal/d' /etc/hosts
echo -e "${LIMA_CIDATA_SLIRP_GATEWAY}\thost.lima.internal" >>/etc/hosts
