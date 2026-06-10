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

LIMACTL_CREATE=(limactl --tty=false create)

CONTAINER_ENGINE="nerdctl"

declare -A CHECKS=(
	["proxy-settings"]="1"
	["systemd"]="1"
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
	["block-device"]=""
	["user-v2"]=""
	["mount-path-with-spaces"]=""
	["provision-data"]=""
	["provision-yq"]=""
	["param-env-variables"]=""
	["set-user"]=""
	["preserve-env"]="1"
	["static-port-forwards"]=""
	["ssh-over-vsock"]=""
)

case "$NAME" in
"default")
	# CI failure:
	# "[hostagent] failed to confirm whether /c/Users/runneradmin [remote] is successfully mounted"
	[ "${OS_HOST}" = "Msys" ] && CHECKS["mount-home"]=
	[ "${OS_HOST}" = "Darwin" ] && CHECKS["ssh-over-vsock"]="1"
	CHECKS["block-device"]=1
	;;
"debian")
	# debian.yaml is the designated template for the block-device test on Linux hosts.
	CHECKS["block-device"]=1
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
"test-misc")
	CHECKS["disk"]=1
	CHECKS["snapshot-online"]="1"
	CHECKS["snapshot-offline"]="1"
	CHECKS["clone"]="1"
	CHECKS["mount-path-with-spaces"]="1"
	CHECKS["provision-data"]="1"
	CHECKS["provision-yq"]="1"
	CHECKS["param-env-variables"]="1"
	CHECKS["set-user"]="1"
	CHECKS["static-port-forwards"]="1"
	;;
"docker")
	CONTAINER_ENGINE="docker"
	;;
"wsl2")
	# TODO https://github.com/lima-vm/lima/issues/3268
	CHECKS["proxy-settings"]=
	;;
"archlinux")
	;;
esac

if limactl ls -q "$NAME" 2>/dev/null; then
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
	limactl shell "$NAME" sh -c "command -v journalctl >/dev/null && sudo journalctl --no-pager" >failure-logs/journal.log
	set +x -e
}

export ftp_proxy=http://localhost:2121

INFO "Creating \"$NAME\" from \"$FILE_HOST\""
defer "limactl delete -f \"$NAME\""

if [[ -n ${CHECKS["disk"]} ]]; then
	if [[ -z "$(limactl disk ls data --json 2>/dev/null)" ]]; then
		defer "limactl disk delete data"
		limactl disk create data --size 10G
	fi
	if ! limactl disk ls | grep -q "^swap\s"; then
		defer "limactl disk delete swap"
		limactl disk create swap --size 2G
	fi
fi

BLOCK_DEVICE=""
if [[ -n ${CHECKS["block-device"]} ]]; then
	INFO "Creating a host block device for the block-device test"
	case "${OS_HOST}" in
	Darwin)
		# The ramdisk is created with sudo so the device node is owned by
		# root, which exercises the privileged helper path instead of the
		# direct open.
		BLOCK_DEVICE="$(sudo hdiutil attach -nomount ram://65536 | awk 'NR==1 {print $1}')"
		defer "sudo hdiutil detach \"${BLOCK_DEVICE}\" -force"
		;;
	"GNU/Linux")
		blockdevice_img="$HOME_HOST/lima-block-device.img"
		rm -f "${blockdevice_img}"
		truncate -s 32M "${blockdevice_img}"
		# The loop device is owned by root, so this exercises the privileged sudo helper.
		BLOCK_DEVICE="$(sudo losetup --find --show "${blockdevice_img}")"
		defer "sudo losetup -d \"${BLOCK_DEVICE}\""
		defer "rm -f \"${blockdevice_img}\""
		;;
	Msys)
		# A VHD attached with diskpart is the one way to conjure a real raw
		# \\.\PhysicalDriveN disk on a stock Windows host: it needs no spare
		# hardware and no Hyper-V PowerShell module. diskpart's /s switch must
		# be shielded from the MSYS2 argument conversion, which would rewrite
		# it as a path. diskpart does not report the new disk number, so it is
		# derived by diffing Get-Disk before and after the attach.
		blockdevice_vhd="$(cygpath -w "$HOME_HOST/lima-block-device.vhd")"
		blockdevice_diskpart="$HOME_HOST/lima-block-device.diskpart.txt"
		printf 'create vdisk file=%s maximum=32 type=fixed\nattach vdisk\n' "${blockdevice_vhd}" >"${blockdevice_diskpart}"
		blockdevice_disks_before="$(powershell.exe -NoProfile -Command '(Get-Disk).Number' | tr -d '\r')"
		MSYS2_ARG_CONV_EXCL='*' diskpart /s "$(cygpath -w "${blockdevice_diskpart}")"
		blockdevice_disks_after="$(powershell.exe -NoProfile -Command '(Get-Disk).Number' | tr -d '\r')"
		blockdevice_number="$(comm -13 <(echo "${blockdevice_disks_before}" | sort) <(echo "${blockdevice_disks_after}" | sort) | head -n1)"
		[ -n "${blockdevice_number}" ]
		printf 'select vdisk file=%s\ndetach vdisk\n' "${blockdevice_vhd}" >"${blockdevice_diskpart}.detach"
		defer "MSYS2_ARG_CONV_EXCL='*' diskpart /s \"$(cygpath -w "${blockdevice_diskpart}.detach")\""
		# Keep the raw disk offline so Windows never uses it concurrently with the guest.
		powershell.exe -NoProfile -Command "Set-Disk -Number ${blockdevice_number} -IsOffline \$true -ErrorAction SilentlyContinue" || true
		# Use the forward-slash DOS device path form: it is equivalent to
		# \\.\PhysicalDriveN for the Windows APIs, but survives the MSYS2
		# argument conversion that mangles leading backslashes.
		BLOCK_DEVICE='//./PhysicalDrive'"${blockdevice_number}"
		;;
	*)
		WARNING "Skipping the block-device test on unsupported host ${OS_HOST}"
		CHECKS["block-device"]=
		;;
	esac
	if [[ -n ${CHECKS["block-device"]} ]]; then
		INFO "Created host block device ${BLOCK_DEVICE}"
		LIMACTL_CREATE_ARGS="${LIMACTL_CREATE_ARGS:-} --block-device=${BLOCK_DEVICE}"
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

if [[ -n ${CHECKS["block-device"]} ]]; then
	# The roundtrip is verified on a raw sector instead of through a
	# filesystem: it proves the guest write reached the actual host device
	# without depending on any filesystem driver being available in the host
	# kernel (CI kernels routinely lack uncommon ones), and without a
	# mount/unmount handoff, which would be unsafe to perform while the
	# instance still holds the device open.
	INFO "Testing that the host block device ${BLOCK_DEVICE} is attached and writable"
	blockdevice_id="$(basename "${BLOCK_DEVICE}")"
	# The virtio-blk serial is derived from the host device basename, so the
	# device shows up under a deterministic /dev/disk/by-id path.
	guest_block_device="/dev/disk/by-id/virtio-${blockdevice_id}"
	if ! limactl shell "$NAME" test -e "${guest_block_device}"; then
		# krunkit cannot set a virtio-blk serial, so locate the device by its size.
		guest_block_device="$(limactl shell "$NAME" lsblk -bdnro PATH,SIZE,TYPE | awk '$3 == "disk" && $2 == 33554432 {print $1; exit}')"
	fi
	INFO "Guest block device: ${guest_block_device}"
	blockdevice_marker="lima-block-device-test-$$"
	limactl shell "$NAME" sudo sh -c "printf '%s' '${blockdevice_marker}' | dd of='${guest_block_device}' bs=512 count=1 conv=sync && sync"
	# Verify the guest write by reading the raw device back on the host.
	case "${OS_HOST}" in
	Darwin | "GNU/Linux")
		# The device node is owned by root on both hosts.
		got="$(sudo dd if="${BLOCK_DEVICE}" bs=512 count=1 2>/dev/null | head -c "${#blockdevice_marker}")"
		;;
	Msys)
		# Reading through MSYS2's /dev/sd<letter> view of \\.\PhysicalDriveN
		# (0 -> a, 1 -> b, ...) keeps the verification in plain dd, identical
		# to the other hosts; Windows itself ships no raw-read tool, and the
		# elevated shell this script requires can read raw disks directly.
		blockdevice_sd="/dev/sd$(echo abcdefghijklmnopqrstuvwxyz | cut -c"$((${BLOCK_DEVICE##*PhysicalDrive} + 1))")"
		got="$(dd if="${blockdevice_sd}" bs=512 count=1 2>/dev/null | head -c "${#blockdevice_marker}")"
		;;
	esac
	INFO "Block device marker: expected=${blockdevice_marker}, got=${got}"
	if [ "${got}" != "${blockdevice_marker}" ]; then
		ERROR "The guest write to ${guest_block_device} did not reach the host device ${BLOCK_DEVICE}"
		exit 1
	fi
fi

if [[ -n ${CHECKS["mount-path-with-spaces"]} ]]; then
	INFO 'Testing that "/tmp/lima test dir with spaces" is not wiped out'
	[ "$(cat "/tmp/lima test dir with spaces/test file")" = "test file content" ]
	[ "$(limactl shell "$NAME" cat "/tmp/lima test dir with spaces/test file")" = "test file content" ]
fi

if [[ -n ${CHECKS["provision-data"]} ]]; then
	INFO 'Testing that /etc/sysctl.d/99-inotify.conf was created successfully on provision'
	limactl shell "$NAME" grep -q fs.inotify.max_user_watches /etc/sysctl.d/99-inotify.conf
fi

if [[ -n ${CHECKS["provision-yq"]} ]]; then
	INFO 'Testing that /tmp/param-yq.json was created successfully on provision'
	limactl shell "$NAME" grep -q '"YQ": "yq"' /tmp/param-yq.json
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

INFO "Testing limactl copy command with scp backend"
tmpfile_scp="$tmpdir/lima-hostname-scp"
rm -f "$tmpfile_scp"
tmpfile_scp_host=$tmpfile_scp
if [ "${OS_HOST}" = "Msys" ]; then
	tmpfile_scp_host="$(cygpath -w "$tmpfile_scp")"
fi
limactl cp --backend=scp "$NAME":/etc/hostname "$tmpfile_scp_host"
expected="$(limactl shell "$NAME" cat /etc/hostname)"
got="$(cat "$tmpfile_scp")"
INFO "/etc/hostname (scp): expected=${expected}, got=${got}"
if [ "$got" != "$expected" ]; then
	ERROR "copy command with scp backend did not fetch the file"
	exit 1
fi

if command -v rsync >/dev/null && limactl shell "$NAME" command -v rsync >/dev/null 2>&1; then
	INFO "Testing limactl copy command with rsync backend"
	tmpfile_rsync="$tmpdir/lima-hostname-rsync"
	rm -f "$tmpfile_rsync"
	tmpfile_rsync_host=$tmpfile_rsync
	if [ "${OS_HOST}" = "Msys" ]; then
		tmpfile_rsync_host="$(cygpath -w "$tmpfile_rsync")"
	fi
	limactl cp --backend=rsync "$NAME":/etc/hostname "$tmpfile_rsync_host"
	expected="$(limactl shell "$NAME" cat /etc/hostname)"
	got="$(cat "$tmpfile_rsync")"
	INFO "/etc/hostname (rsync): expected=${expected}, got=${got}"
	if [ "$got" != "$expected" ]; then
		ERROR "copy command with rsync backend did not fetch the file"
		exit 1
	fi

	INFO "Testing limactl copy command with rsync backend (verbose, recursive)"
	testdir="$tmpdir/test-rsync-dir"
	mkdir -p "$testdir"
	echo "test content" >"$testdir/testfile.txt"
	limactl cp --backend=rsync -r -v "$testdir" "$NAME":/tmp/
	if ! limactl shell "$NAME" test -f /tmp/test-rsync-dir/testfile.txt; then
		ERROR "rsync recursive copy failed"
		exit 1
	fi
	rsync_content="$(limactl shell "$NAME" cat /tmp/test-rsync-dir/testfile.txt)"
	if [ "$rsync_content" != "test content" ]; then
		ERROR "rsync file content mismatch"
		exit 1
	fi
else
	INFO "Skipping rsync backend test (rsync not available on host or guest)"
fi

INFO "Testing limactl command with escaped characters"
limactl shell "$NAME" bash -c "$(echo -e '\n\techo foo\n\techo bar')"

INFO "Testing limactl command with quotes"
limactl shell "$NAME" bash -c "echo 'foo \"bar\"'"

if [[ -n ${CHECKS["systemd"]} ]]; then
	set -x
	if ! limactl shell "$NAME" systemctl is-system-running --wait; then
		ERROR '"systemctl is-system-running" failed'
		diagnose "$NAME"
		exit 1
	fi
	set +x
fi

if [[ -n ${CHECKS["mount-home"]} ]]; then
	"${scriptdir}"/test-mount-home.sh "$NAME"
fi

if [[ -n ${CHECKS["ssh-over-vsock"]} ]]; then
	if [[ "$(limactl ls "${NAME}" --yq .vmType)" == "vz" ]]; then
		INFO "Testing SSH over vsock"
		set -x
		log_file="$HOME_HOST/.lima/${NAME}/ha.stdout.log"

		# Helper function to check vsock events in the log file
		# $1: event_type to check for
		check_vsock_event() {
			local event_type="$1"
			if jq -e --arg type "$event_type" 'select(.status.vsock.type == $type)' "$log_file" >/dev/null 2>&1; then
				return 0
			fi
			return 1
		}

		INFO "Testing .ssh.overVsock=true configuration"
		limactl stop "${NAME}"
		# Detection of the SSH server on VSOCK may fail; however, a failing log indicates that controlling detection via the environment variable works as expected.
		limactl start --set '.ssh.overVsock=true' "${NAME}"
		if ! check_vsock_event "started" && ! check_vsock_event "failed"; then
			set +x
			diagnose "${NAME}"
			ERROR ".ssh.overVsock=true did not enable vsock forwarder"
			exit 1
		fi
		INFO 'Testing .ssh.overVsock=null configuration'
		limactl stop "${NAME}"
		# Detection of the SSH server on VSOCK may fail; however, a failing log indicates that controlling detection via the environment variable works as expected.
		limactl start --set '.ssh.overVsock=null' "${NAME}"
		if ! check_vsock_event "started" && ! check_vsock_event "failed"; then
			set +x
			diagnose "${NAME}"
			ERROR ".ssh.overVsock=null did not enable vsock forwarder"
			exit 1
		fi
		INFO "Testing .ssh.overVsock=false configuration"
		limactl stop "${NAME}"
		limactl start --set '.ssh.overVsock=false' "${NAME}"
		if ! check_vsock_event "skipped"; then
			set +x
			diagnose "${NAME}"
			ERROR ".ssh.overVsock=false did not disable vsock forwarder"
			exit 1
		fi
		set +x
	fi
fi

# Use GHCR and ECR to avoid hitting Docker Hub rate limit
nginx_image="ghcr.io/stargz-containers/nginx:1.19-alpine-org"
alpine_image="ghcr.io/containerd/alpine:3.14.0"
coredns_image="public.ecr.aws/eks-distro/coredns/coredns:v1.12.2-eks-1-31-latest"

if [[ -n ${CHECKS["container-engine"]} ]]; then
	sudo=""
	# Currently WSL2 machines only support privileged engine. This requirement might be lifted in the future.
	if [[ "$(limactl ls "${NAME}" --yq .vmType)" == "wsl2" ]]; then
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

	if [ "${OS_HOST}" != "Msys" ]; then
		# TODO: support UDP on Windows
		INFO "Run a coredns container with port forwarding 127.0.0.1:10053/udp"
		limactl shell "$NAME" $sudo $CONTAINER_ENGINE pull --quiet ${coredns_image}
		limactl shell "$NAME" $sudo $CONTAINER_ENGINE run -d --name coredns -p 127.0.0.1:10053:53/udp ${coredns_image}
		dig @127.0.0.1 -p 10053 lima-vm.io
		limactl shell "$NAME" $sudo $CONTAINER_ENGINE rm -f coredns
	fi

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
	PORT_FORWARDING_CONNECTION_TIMEOUT=1
	INFO "Testing port forwarding rules using netcat and socat with connection timeout ${PORT_FORWARDING_CONNECTION_TIMEOUT}s"
	set -x
	if [[ ${NAME} == "alpine"* ]]; then
		limactl shell "${NAME}" sudo apk add socat
	fi
	if [[ ${NAME} == "archlinux" ]]; then
		limactl shell "${NAME}" sudo pacman -Syu --noconfirm openbsd-netcat socat
	fi
	if [[ ${NAME} == "debian" || ${NAME} == "default" || ${NAME} == "docker" || ${NAME} == "test-misc" ]]; then
		limactl shell "${NAME}" sudo apt-get install -y netcat-openbsd socat
	fi
	if [[ ${NAME} == "fedora" || ${NAME} == "wsl2" ]]; then
		limactl shell "${NAME}" sudo dnf install -y nc socat
	fi
	if [[ ${NAME} == "opensuse" ]]; then
		limactl shell "${NAME}" sudo zypper in -y netcat-openbsd socat
	fi
	if limactl shell "${NAME}" command -v dnf; then
		limactl shell "${NAME}" sudo dnf install -y nc socat
	fi
	if "${scriptdir}/test-port-forwarding.pl" "${NAME}" socat $PORT_FORWARDING_CONNECTION_TIMEOUT; then
		INFO "Port forwarding rules work"
	else
		ERROR "Port forwarding rules do not work with socat"
		diagnose "$NAME"
		exit 1
	fi

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
			if [[ "$(limactl ls "${NAME}" --yq .vmType)" == "wsl2" ]]; then
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
		if [[ ${NAME} != "alpine"* ]] && command -v w3m >/dev/null; then
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
	if ! limactl shell "$NAME" lsblk --output NAME,MOUNTPOINT | grep -q "\[SWAP\]"; then
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
	limactl delete -f "$secondvm" >/dev/null 2>&1 || true
	"${LIMACTL_CREATE[@]}" --set ".additionalDisks=null" "$FILE_HOST" --name "$secondvm"
	defer "limactl delete -f \"$secondvm\" >/dev/null 2>&1 || true"
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

if [[ $NAME == "fedora" && "$(limactl ls "${NAME}" --yq .vmType)" == "vz" ]]; then
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

if [[ -n ${CHECKS["static-port-forwards"]} ]]; then
	INFO "Testing static port forwarding functionality"
	"${scriptdir}/test-plain-static-port-forward.sh" "$NAME"
	"${scriptdir}/test-nonplain-static-port-forward.sh" "$NAME"
	INFO "All static port forwarding tests passed!"
fi
