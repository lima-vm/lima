#!/usr/bin/env bash

set -eu -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"

if [ "$#" -ne 1 ]; then
	ERROR "Usage: $0 NAME"
	exit 1
fi

NAME="$1"
##########################################################################################
## When using vz & virtiofs, initially container_file_t selinux label
## was considered which works perfectly for container work loads
## but it might break for other work loads if the process is running with
## different label. Also these are the remote mounts from the host machine,
## so keeping the label as nfs_t fits right. Package container-selinux by
## default adds rules for nfs_t context which allows container workloads to work as well.
## https://github.com/lima-vm/lima/pull/1965
##########################################################################################
expected="context=system_u:object_r:nfs_t:s0"
#Skip Rosetta checks for x86 GHA mac runners
if [[ "$(uname)" == "Darwin" && "$(arch)" == "arm64" ]]; then
	INFO "Testing secontext is set for rosetta mounts"
	got=$(limactl shell "$NAME" mount | grep "rosetta" | awk '{print $6}')
	INFO "secontext rosetta: expected=${expected}, got=${got}"
	if [[ $got != *$expected* ]]; then
		ERROR "secontext for rosetta mount is not set or Invalid"
		exit 1
	fi
fi
INFO "Testing secontext is set for bind mounts"
INFO "Checking in mounts"
got=$(limactl shell "$NAME" mount | grep "$HOME" | awk '{print $6}')
INFO "secontext ${HOME}: expected=${expected}, got=${got}"
if [[ $got != *$expected* ]]; then
	ERROR "secontext for \"$HOME\" dir is not set or Invalid"
	exit 1
fi
got=$(limactl shell "$NAME" mount | grep "/tmp/lima" | awk '{print $6}')
INFO "secontext /tmp/lima: expected=${expected}, got=${got}"
if [[ $got != *$expected* ]]; then
	ERROR 'secontext for "/tmp/lima" dir is not set or Invalid'
	exit 1
fi
INFO "Checking in fstab file"
expected='context="system_u:object_r:nfs_t:s0"'
got=$(limactl shell "$NAME" cat /etc/fstab | grep "$HOME" | awk '{print $4}')
INFO "secontext ${HOME}: expected=${expected}, got=${got}"
if [[ $got != *$expected* ]]; then
	ERROR "secontext for \"$HOME\" dir is not set or Invalid"
	exit 1
fi
got=$(limactl shell "$NAME" cat /etc/fstab | grep "/tmp/lima" | awk '{print $4}')
INFO "secontext /tmp/lima: expected=${expected}, got=${got}"
if [[ $got != *$expected* ]]; then
	ERROR 'secontext for "/tmp/lima" dir is not set or Invalid'
	exit 1
fi
