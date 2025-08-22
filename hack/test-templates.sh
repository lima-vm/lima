#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

# will prevent msys2 converting Linux path arguments into Windows paths before passing to limactl
export MSYS2_ARG_CONV_EXCL='*'

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"

if [ "$#" -ne 1 ]; then
	ERROR "Usage: $0 FILE.yaml"
	exit 1
fi

# Resolve any ../ fragments in the filename because they are invalid in relative template locators
FILE="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
NAME="$(basename -s .yaml "$FILE")"
OS_HOST="$(uname -o)"

# On Windows $HOME of the bash runner, %USERPROFILE% of the host machine and mounting point in the guest machine
# are all different folders. This will handle path differences, when values are explicitly set.
HOME_HOST=${HOME_HOST:-$HOME}
HOME_GUEST=${HOME_GUEST:-$HOME}
FILE_HOST=$FILE
if [ "${OS_HOST}" = "Msys" ]; then
	FILE_HOST="$(cygpath -w "$FILE")"
fi

INFO "Validating \"$FILE_HOST\""
limactl validate "$FILE_HOST"

# --cpus=1 is needed for running vz on GHA: https://github.com/lima-vm/lima/pull/1511#issuecomment-1574937888
LIMACTL_CREATE=(limactl --tty=false create --cpus=1 --memory=1)

CONTAINER_ENGINE="nerdctl"

declare -A CHECKS=(
	["proxy-settings"]="1"
	["systemd"]="1"
	["systemd-strict"]="1"
	["mount-home"]="1"
	["container-engine"]="1"
	["restart"]="1"
	# snapshot tests are too flaky (especially with archlinux)
	["snapshot-online"]=""
	["snapshot-offline"]=""
	["clone"]=""
	["port-forwards"]="1"
	["vmnet"]=""
	["disk"]=""
	["user-v2"]=""
	["mount-path-with-spaces"]=""
	["provision-data"]=""
	["param-env-variables"]=""
	["set-user"]=""
)

case "$NAME" in
"default")
	# CI failure:
	# "[hostagent] failed to confirm whether /c/Users/runneradmin [remote] is successfully mounted"
	[ "${OS_HOST}" = "Msys" ] && CHECKS["mount-home"]=
	;;
"alpine"*)
	WARNING "Alpine does not support systemd"
	CHECKS["systemd"]=
	CHECKS["container-engine"]=
	[ "$NAME" = "alpine-iso-9p-writable" ] && CHECKS["mount-path-with-spaces"]="1"
	;;
"k3s")
	ERROR "File \"$FILE\" is not testable with this script"
	exit 1
	;;
"fedora")
	WARNING "Relaxing systemd tests for fedora (For avoiding CI failure)"
	# CI failure:
	# ● run-r2b459797f5b04262bfa79984077a65c7.service                                       loaded failed failed    /usr/bin/systemctl start man-db-cache-update
	CHECKS["systemd-strict"]=
	;;
"test-misc")
	CHECKS["disk"]=1
	CHECKS["snapshot-online"]="1"
	CHECKS["snapshot-offline"]="1"
	CHECKS["clone"]="1"
	CHECKS["mount-path-with-spaces"]="1"
	CHECKS["provision-data"]="1"
	CHECKS["param-env-variables"]="1"
	CHECKS["set-user"]="1"
	;;
"docker")
	CONTAINER_ENGINE="docker"
	;;
"wsl2")
	# TODO https://github.com/lima-vm/lima/issues/3268
	CHECKS["proxy-settings"]=
	;;
esac

if limactl ls -q | grep -q "$NAME"; then
	ERROR "Instance $NAME already exists"
	exit 1
fi

case "$(limactl tmpl yq "$FILE_HOST" '.networks[].lima')" in
"shared")
	CHECKS["vmnet"]=1
	;;
"user-v2")
	CHECKS["port-forwards"]=""
	CHECKS["user-v2"]=1
	;;
esac

if [[ -n ${CHECKS["port-forwards"]} ]]; then
	tmpconfig="$HOME_HOST/lima-config-tmp"
	mkdir -p "${tmpconfig}"
	defer "rm -rf \"$tmpconfig\""
	tmpfile="${tmpconfig}/${NAME}.yaml"
	cp "$FILE" "${tmpfile}"
	FILE="${tmpfile}"
	FILE_HOST=$FILE
	if [ "${OS_HOST}" = "Msys" ]; then
		FILE_HOST="$(cygpath -w "$FILE")"
	fi

	INFO "Setup port forwarding rules for testing in \"${FILE}\""
	"${scriptdir}/test-port-forwarding.pl" "${FILE}"
	INFO "Validating \"$FILE_HOST\""
	limactl validate "$FILE_HOST"
fi

INFO "Make sure template embedding copies \"$FILE_HOST\" exactly"
diff -u <(echo -n "base: $FILE_HOST" | limactl tmpl copy --embed - -) "$FILE_HOST"

function diagnose() {
	NAME="$1"
	set -x +e
	tail "$HOME_HOST/.lima/${NAME}"/*.log
	limactl shell "$NAME" systemctl --no-pager status
	limactl shell "$NAME" systemctl --no-pager
	mkdir -p failure-logs
	cp -pf "$HOME_HOST/.lima/${NAME}"/*.log failure-logs/
	limactl shell "$NAME" sudo cat /var/log/cloud-init-output.log | tee failure-logs/cloud-init-output.log
	set +x -e
}

export ftp_proxy=http://localhost:2121

INFO "Creating \"$NAME\" from \"$FILE_HOST\""
defer "limactl delete -f \"$NAME\""

if [[ -n ${CHECKS["disk"]} ]]; then
	if ! limactl disk ls | grep -q "^data\s"; then
		defer "limactl disk delete data"
		limactl disk create data --size 10G
	fi
fi

set -x
# shellcheck disable=SC2086
"${LIMACTL_CREATE[@]}" ${LIMACTL_CREATE_ARGS:-} "$FILE_HOST"
set +x

if [[ -n ${CHECKS["mount-path-with-spaces"]} ]]; then
	mkdir -p "/tmp/lima test dir with spaces"
	echo "test file content" >"/tmp/lima test dir with spaces/test file"
fi

INFO "Starting \"$NAME\""
set -x
if ! limactl start "$NAME"; then
	ERROR "Failed to start \"$NAME\""
	diagnose "$NAME"
	exit 1
fi

limactl shell "$NAME" uname -a

limactl shell "$NAME" cat /etc/os-release
set +x

INFO "Testing that host home is not wiped out"
[ -e "$HOME_HOST/.lima" ]

if [[ -n ${CHECKS["mount-path-with-spaces"]} ]]; then
	INFO 'Testing that "/tmp/lima test dir with spaces" is not wiped out'
	[ "$(cat "/tmp/lima test dir with spaces/test file")" = "test file content" ]
	[ "$(limactl shell "$NAME" cat "/tmp/lima test dir with spaces/test file")" = "test file content" ]
fi

if [[ -n ${CHECKS["provision-data"]} ]]; then
	INFO 'Testing that /etc/sysctl.d/99-inotify.conf was created successfully on provision'
	limactl shell "$NAME" grep -q fs.inotify.max_user_watches /etc/sysctl.d/99-inotify.conf
fi

if [[ -n ${CHECKS["param-env-variables"]} ]]; then
	INFO 'Testing that PARAM env variables are exported to all types of provisioning scripts and probes'
	limactl shell "$NAME" test -e /tmp/param-ansible
	limactl shell "$NAME" test -e /tmp/param-boot
	limactl shell "$NAME" test -e /tmp/param-dependency
	limactl shell "$NAME" test -e /tmp/param-probe
	limactl shell "$NAME" test -e /tmp/param-system
	limactl shell "$NAME" test -e /tmp/param-user
fi

if [[ -n ${CHECKS["set-user"]} ]]; then
	INFO 'Testing that user settings can be provided by lima.yaml'
	limactl shell "$NAME" grep "^john:x:4711:4711:John Doe:/home/john-john:/usr/bin/bash" /etc/passwd
fi

if [[ -n ${CHECKS["proxy-settings"]} ]]; then
	INFO "Testing proxy settings are imported"
	got=$(limactl shell "$NAME" env | grep FTP_PROXY)
	# Expected: FTP_PROXY is set in addition to ftp_proxy, localhost is replaced
	# by the gateway address, and the value is set immediately without a restart
	gatewayIp=$(limactl shell "$NAME" ip route show 0.0.0.0/0 dev eth0 | cut -d\  -f3)
	expected="FTP_PROXY=http://${gatewayIp}:2121"
	INFO "FTP_PROXY: expected=${expected} got=${got}"
	if [ "$got" != "$expected" ]; then
		ERROR "proxy environment variable not set to correct value"
		exit 1
	fi
fi

INFO "Testing limactl copy command"
tmpdir="$(mktemp -d "${TMPDIR:-/tmp}"/lima-test-templates.XXXXXX)"
defer "rm -rf \"$tmpdir\""
tmpfile="$tmpdir/lima-hostname"
rm -f "$tmpfile"
tmpfile_host=$tmpfile
if [ "${OS_HOST}" = "Msys" ]; then
	tmpfile_host="$(cygpath -w "$tmpfile")"
fi
limactl cp "$NAME":/etc/hostname "$tmpfile_host"
expected="$(limactl shell "$NAME" cat /etc/hostname)"
got="$(cat "$tmpfile")"
INFO "/etc/hostname: expected=${expected}, got=${got}"
if [ "$got" != "$expected" ]; then
	ERROR "copy command did not fetch the file"
	exit 1
fi

INFO "Testing limactl command with escaped characters"
limactl shell "$NAME" bash -c "$(echo -e '\n\techo foo\n\techo bar')"

INFO "Testing limactl command with quotes"
limactl shell "$NAME" bash -c "echo 'foo \"bar\"'"

if [[ -n ${CHECKS["systemd"]} ]]; then
	set -x
	# agetty segfaults on Ubuntu 25.04 (x86_64)
	# > __strncmp_evex () at ../sysdeps/x86_64/multiarch/strcmp-evex.S:316
	# Should be fixed in Ubuntu 25.10 with util-linux >= 2.41
	# https://github.com/lima-vm/lima/pull/3643#issuecomment-3006788732
	if limactl shell "$NAME" systemctl -q is-failed serial-getty@ttyS0.service; then
		limactl shell "$NAME" sudo systemctl reset-failed serial-getty@ttyS0.service
	fi
	if ! limactl shell "$NAME" systemctl is-system-running --wait; then
		ERROR '"systemctl is-system-running" failed'
		diagnose "$NAME"
		if [[ -z ${CHECKS["systemd-strict"]} ]]; then
			INFO 'Ignoring "systemctl is-system-running" failure'
		else
			exit 1
		fi
	fi
	set +x
fi

if [[ -n ${CHECKS["mount-home"]} ]]; then
	"${scriptdir}"/test-mount-home.sh "$NAME"
fi

# Use GHCR to avoid hitting Docker Hub rate limit
nginx_image="ghcr.io/stargz-containers/nginx:1.19-alpine-org"
alpine_image="ghcr.io/containerd/alpine:3.14.0"

if [[ -n ${CHECKS["container-engine"]} ]]; then
	sudo=""
	# Currently WSL2 machines only support privileged engine. This requirement might be lifted in the future.
	if [[ "$(limactl ls --json "${NAME}" | jq -r .vmType)" == "wsl2" ]]; then
		sudo="sudo"
	fi
	INFO "Run a nginx container with port forwarding 127.0.0.1:8080"
	set -x
	if ! limactl shell "$NAME" $sudo $CONTAINER_ENGINE info; then
		limactl shell "$NAME" cat /var/log/cloud-init-output.log
		ERROR "\"${CONTAINER_ENGINE} info\" failed"
		exit 1
	fi
	limactl shell "$NAME" $sudo $CONTAINER_ENGINE pull --quiet ${nginx_image}
	limactl shell "$NAME" $sudo $CONTAINER_ENGINE run -d --name nginx -p 127.0.0.1:8080:80 ${nginx_image}

	timeout 3m bash -euxc "until curl -f --retry 30 --retry-connrefused http://127.0.0.1:8080; do sleep 3; done"

	limactl shell "$NAME" $sudo $CONTAINER_ENGINE rm -f nginx
	set +x
	if [[ -n ${CHECKS["mount-home"]} ]]; then
		hometmp="$HOME_HOST/lima-container-engine-test-tmp"
		hometmpguest="$HOME_GUEST/lima-container-engine-test-tmp"
		# test for https://github.com/lima-vm/lima/issues/187
		INFO "Testing home bind mount (\"$hometmp\")"
		rm -rf "$hometmp"
		mkdir -p "$hometmp"
		defer "rm -rf \"$hometmp\""
		set -x
		limactl shell "$NAME" $sudo $CONTAINER_ENGINE pull --quiet ${alpine_image}
		echo "random-content-${RANDOM}" >"$hometmp/random"
		expected="$(cat "$hometmp/random")"
		got="$(limactl shell "$NAME" $sudo $CONTAINER_ENGINE run --rm -v "$hometmpguest/random":/mnt/foo ${alpine_image} cat /mnt/foo)"
		INFO "$hometmp/random: expected=${expected}, got=${got}"
		if [ "$got" != "$expected" ]; then
			ERROR "Home directory is not shared?"
			exit 1
		fi
		set +x
	fi
fi

if [[ -n ${CHECKS["port-forwards"]} ]]; then
	INFO "Testing port forwarding rules using netcat"
	set -x
	if [ "${NAME}" = "archlinux" ]; then
		limactl shell "$NAME" sudo pacman -Syu --noconfirm openbsd-netcat
	fi
	if [ "${NAME}" = "debian" ]; then
		limactl shell "$NAME" sudo apt-get install -y netcat-openbsd
	fi
	if [ "${NAME}" == "fedora" ]; then
		limactl shell "$NAME" sudo dnf install -y nc
	fi
	if [ "${NAME}" = "opensuse" ]; then
		limactl shell "$NAME" sudo zypper in -y netcat-openbsd
	fi
	if limactl shell "$NAME" command -v dnf; then
		limactl shell "$NAME" sudo dnf install -y nc
	fi
	"${scriptdir}/test-port-forwarding.pl" "${NAME}"

	if [[ -n ${CHECKS["container-engine"]} || ${NAME} == "alpine"* ]]; then
		INFO "Testing that \"${CONTAINER_ENGINE} run\" binds to 0.0.0.0 and is forwarded to the host (non-default behavior, configured via test-port-forwarding.pl)"
		if [ "$(uname)" = "Darwin" ]; then
			# macOS runners seem to use `localhost` as the hostname, so the perl lookup just returns `127.0.0.1`
			hostip=$(system_profiler SPNetworkDataType -json | jq -r 'first(.SPNetworkDataType[] | select(.ip_address) | .ip_address) | first')
		else
			hostip=$(perl -MSocket -MSys::Hostname -E 'say inet_ntoa(scalar gethostbyname(hostname()))')
		fi
		if [ -n "${hostip}" ]; then
			sudo=""
			if [[ ${NAME} == "alpine"* ]]; then
				arch=$(limactl info | jq -r .defaultTemplate.arch)
				nerdctl=$(limactl info | jq -r ".defaultTemplate.containerd.archives[] | select(.arch==\"$arch\").location")
				curl -Lso nerdctl-full.tgz "${nerdctl}"
				limactl shell "$NAME" sudo apk add containerd
				limactl shell "$NAME" sudo rc-service containerd start
				limactl shell "$NAME" sudo tar xzf "${PWD}/nerdctl-full.tgz" -C /usr/local
				rm nerdctl-full.tgz
				sudo="sudo"
			fi
			# Currently WSL2 machines only support privileged engine. This requirement might be lifted in the future.
			if [[ "$(limactl ls --json "${NAME}" | jq -r .vmType)" == "wsl2" ]]; then
				sudo="sudo"
			fi
			limactl shell "$NAME" $sudo $CONTAINER_ENGINE info
			limactl shell "$NAME" $sudo $CONTAINER_ENGINE pull --quiet ${nginx_image}

			limactl shell "$NAME" $sudo $CONTAINER_ENGINE run -d --name nginx -p 8888:80 ${nginx_image}
			timeout 3m bash -euxc "until curl -f --retry 30 --retry-connrefused http://${hostip}:8888; do sleep 3; done"
			limactl shell "$NAME" $sudo $CONTAINER_ENGINE rm -f nginx

			if [ "$(uname)" = "Darwin" ]; then
				# Only macOS can bind to port 80 without root
				limactl shell "$NAME" $sudo $CONTAINER_ENGINE run -d --name nginx -p 127.0.0.1:80:80 ${nginx_image}
				timeout 3m bash -euxc "until curl -f --retry 30 --retry-connrefused http://localhost:80; do sleep 3; done"
				limactl shell "$NAME" $sudo $CONTAINER_ENGINE rm -f nginx
			fi
		fi
		if [[ ${NAME} != "alpine"* && ${NAME} != "wsl2"* ]] && command -v w3m >/dev/null; then
			INFO "Testing https://github.com/lima-vm/lima/issues/3685 ([gRPC portfwd] client connection is not closed immediately when server closed the connection)"
			# Skip the test on Alpine, as systemd-run is missing
			# Skip the test on WSL2, as port forwarding is half broken https://github.com/lima-vm/lima/pull/3686#issuecomment-3034842616
			limactl shell "$NAME" systemd-run --user python3 -m http.server 3685
			# curl is not enough to reproduce https://github.com/lima-vm/lima/issues/3685
			# `w3m -dump` exits with status code 0 even on "Can't load" error.
			timeout 30s bash -euxc "until w3m -dump http://localhost:3685 | grep -v \"w3m: Can't load\"; do sleep 3; done"
		fi
	fi
	set +x
fi

if [[ -n ${CHECKS["vmnet"]} ]]; then
	INFO "Testing vmnet functionality"
	guestip="$(limactl shell "$NAME" ip -4 -j addr show dev lima0 | jq -r '.[0].addr_info[0].local')"
	INFO "Pinging the guest IP ${guestip}"
	set -x
	ping -c 3 "$guestip"
	set +x
	INFO "Benchmarking with iperf3"
	set -x
	limactl shell "$NAME" sudo DEBIAN_FRONTEND=noninteractive apt-get install -y iperf3
	limactl shell "$NAME" iperf3 -s -1 -D
	${IPERF3} -c "$guestip"
	set +x
	# NOTE: we only test the shared interface here, as the bridged interface cannot be used on GHA (and systemd-networkd-wait-online.service will fail)
fi

if [[ -n ${CHECKS["disk"]} ]]; then
	INFO "Testing disk is attached"
	set -x
	if ! limactl shell "$NAME" lsblk --output NAME,MOUNTPOINT | grep -q "/mnt/lima-data"; then
		ERROR "Disk is not mounted"
		exit 1
	fi
	set +x
fi

if [[ -n ${CHECKS["restart"]} ]]; then
	INFO "Create file in the guest home directory and verify that it still exists after a restart"
	# shellcheck disable=SC2016
	limactl shell "$NAME" sh -c 'touch $HOME/sweet-home'
	if [[ -n ${CHECKS["disk"]} ]]; then
		INFO "Create file in disk and verify that it still exists when it is reattached"
		limactl shell "$NAME" sudo sh -c 'touch /mnt/lima-data/sweet-disk'
	fi

	INFO "Stopping \"$NAME\""
	limactl stop "$NAME"
	sleep 3

	if [[ -n ${CHECKS["disk"]} ]]; then
		INFO "Resize disk and verify that partition and fs size are increased"
		limactl disk resize data --size 11G
	fi

	export ftp_proxy=my.proxy:8021
	INFO "Restarting \"$NAME\""
	if ! limactl start "$NAME"; then
		ERROR "Failed to start \"$NAME\""
		diagnose "$NAME"
		exit 1
	fi

	INFO "Make sure proxy setting is updated"
	got=$(limactl shell "$NAME" env | grep FTP_PROXY)
	expected="FTP_PROXY=my.proxy:8021"
	INFO "FTP_PROXY: expected=${expected} got=${got}"
	if [ "$got" != "$expected" ]; then
		ERROR "proxy environment variable not set to correct value"
		exit 1
	fi

	# shellcheck disable=SC2016
	if ! limactl shell "$NAME" sh -c 'test -f $HOME/sweet-home'; then
		ERROR "Guest home directory does not persist across restarts"
		exit 1
	fi

	if [[ -n ${CHECKS["disk"]} ]]; then
		if ! limactl shell "$NAME" sh -c 'test -f /mnt/lima-data/sweet-disk'; then
			ERROR "Disk does not persist across restarts"
			exit 1
		fi
		if ! limactl shell "$NAME" sh -c 'df -h /mnt/lima-data/ --output=size | grep -q 11G'; then
			ERROR "Disk FS does not resized after restart"
			exit 1
		fi
	fi
fi

if [[ -n ${CHECKS["user-v2"]} ]]; then
	INFO "Testing user-v2 network"
	secondvm="$NAME-1"
	"${LIMACTL_CREATE[@]}" --set ".additionalDisks=null" "$FILE_HOST" --name "$secondvm"
	if ! limactl start "$secondvm"; then
		ERROR "Failed to start \"$secondvm\""
		diagnose "$secondvm"
		exit 1
	fi
	secondvmDNS="lima-$secondvm.internal"
	INFO "DNS of $secondvm is $secondvmDNS"
	set -x
	if ! limactl shell "$NAME" ping -c 1 "$secondvmDNS"; then
		ERROR "Failed to do vm->vm communication via user-v2"
		INFO "Stopping \"$secondvm\""
		limactl stop "$secondvm"
		INFO "Deleting \"$secondvm\""
		limactl delete "$secondvm"
		exit 1
	fi
	INFO "Stopping \"$secondvm\""
	limactl stop "$secondvm"
	INFO "Deleting \"$secondvm\""
	limactl delete "$secondvm"
	set +x
fi
if [[ -n ${CHECKS["snapshot-online"]} ]]; then
	INFO "Testing online snapshots"
	limactl shell "$NAME" sh -c 'echo foo > /tmp/test'
	limactl snapshot create "$NAME" --tag snap1
	got=$(limactl snapshot list "$NAME" --quiet)
	expected="snap1"
	INFO "snapshot list: expected=${expected} got=${got}"
	if [ "$got" != "$expected" ]; then
		ERROR "snapshot list did not return expected value"
		exit 1
	fi
	limactl shell "$NAME" sh -c 'echo bar > /tmp/test'
	limactl snapshot apply "$NAME" --tag snap1
	got=$(limactl shell "$NAME" cat /tmp/test)
	expected="foo"
	INFO "snapshot apply: expected=${expected} got=${got}"
	if [ "$got" != "$expected" ]; then
		ERROR "snapshot apply did not restore snapshot"
		exit 1
	fi
	limactl snapshot delete "$NAME" --tag snap1
	limactl shell "$NAME" rm /tmp/test
fi
if [[ -n ${CHECKS["snapshot-offline"]} ]]; then
	INFO "Testing offline snapshots"
	limactl stop "$NAME"
	sleep 3
	limactl snapshot create "$NAME" --tag snap2
	got=$(limactl snapshot list "$NAME" --quiet)
	expected="snap2"
	INFO "snapshot list: expected=${expected} got=${got}"
	if [ "$got" != "$expected" ]; then
		ERROR "snapshot list did not return expected value"
		exit 1
	fi
	limactl snapshot apply "$NAME" --tag snap2
	limactl snapshot delete "$NAME" --tag snap2
	limactl start "$NAME"
fi
if [[ -n ${CHECKS["clone"]} ]]; then
	INFO "Testing cloning"
	limactl stop "$NAME"
	sleep 3
	# [hostagent] could not attach disk \"data\", in use by instance \"test-misc-clone\"
	limactl clone --set '.additionalDisks = null' "$NAME" "${NAME}-clone"
	limactl start "${NAME}-clone"
	[ "$(limactl shell "${NAME}-clone" hostname)" = "lima-${NAME}-clone" ]
	limactl start "$NAME"
fi

if [[ $NAME == "fedora" && "$(limactl ls --json "$NAME" | jq -r .vmType)" == "vz" ]]; then
	"${scriptdir}"/test-selinux.sh "$NAME"
fi

INFO "Stopping \"$NAME\""
limactl stop "$NAME"
sleep 3

INFO "Deleting \"$NAME\""
limactl delete "$NAME"

if [[ -n ${CHECKS["mount-path-with-spaces"]} ]]; then
	rm -rf "/tmp/lima test dir with spaces"
fi
