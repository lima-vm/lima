#!/usr/bin/env bash
set -eu -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.inc.sh
source "${scriptdir}/common.inc.sh"

if [ "$#" -ne 1 ]; then
	ERROR "Usage: $0 FILE.yaml"
	exit 1
fi

FILE="$1"
NAME="$(basename -s .yaml "$FILE")"

INFO "Validating \"$FILE\""
limactl validate "$FILE"

declare -A CHECKS=(
	["systemd"]="1"
	["systemd-strict"]="1"
	["mount-home"]="1"
	["containerd-user"]="1"
	["restart"]="1"
	["port-forwards"]="1"
	["vmnet"]=""
)

case "$NAME" in
"alpine")
	WARNING "Alpine does not support systemd"
	CHECKS["systemd"]=
	CHECKS["containerd-user"]=
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
"vmnet")
	CHECKS["vmnet"]=1
	;;
esac

if limactl ls -q | grep -q "$NAME"; then
	ERROR "Instance $NAME already exists"
	exit 1
fi

if [[ -n ${CHECKS["port-forwards"]} ]]; then
	tmpconfig="$HOME/lima-config-tmp"
	mkdir -p "${tmpconfig}"
	defer "rm -rf \"$tmpconfig\""
	tmpfile="${tmpconfig}/${NAME}.yaml"
	cp "$FILE" "${tmpfile}"
	FILE="${tmpfile}"
	INFO "Setup port forwarding rules for testing in \"${FILE}\""
	"${scriptdir}/test-port-forwarding.pl" "${FILE}"
	limactl validate "$FILE"
fi

function diagnose() {
	NAME="$1"
	set -x +e
	tail "$HOME/.lima/${NAME}"/*.log
	limactl shell "$NAME" systemctl status
	limactl shell "$NAME" systemctl
	limactl shell "$NAME" sudo cat /var/log/cloud-init-output.log
	set +x -e
}

export ftp_proxy=http://localhost:2121

INFO "Starting \"$NAME\" from \"$FILE\""
defer "limactl delete -f \"$NAME\""
set -x
if ! limactl start --tty=false "$FILE"; then
	ERROR "Failed to start \"$NAME\""
	diagnose "$NAME"
	exit 1
fi

limactl shell "$NAME" uname -a

limactl shell "$NAME" cat /etc/os-release
set +x

INFO "Testing proxy settings are imported"
got=$(limactl shell "$NAME" env | grep FTP_PROXY)
# Expected: FTP_PROXY is set in addition to ftp_proxy, localhost is replaced
# by the gateway address, and the value is set immediately without a restart
expected="FTP_PROXY=http://192.168.5.2:2121"
INFO "FTP_PROXY: expected=${expected} got=${got}"
if [ "$got" != "$expected" ]; then
	ERROR "proxy environment variable not set to correct value"
	exit 1
fi

INFO "Testing limactl copy command"
tmpfile="$HOME/lima-hostname"
rm -f "$tmpfile"
limactl cp "$NAME":/etc/hostname "$tmpfile"
defer "rm -f \"$tmpfile\""
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

if [[ -n ${CHECKS["containerd-user"]} ]]; then
	INFO "Run a nginx container with port forwarding 127.0.0.1:8080"
	set -x
	limactl shell "$NAME" nerdctl info
	limactl shell "$NAME" nerdctl pull --quiet ${nginx_image}
	limactl shell "$NAME" nerdctl run -d --name nginx -p 127.0.0.1:8080:80 ${nginx_image}

	timeout 3m bash -euxc "until curl -f --retry 30 --retry-connrefused http://127.0.0.1:8080; do sleep 3; done"

	limactl shell "$NAME" nerdctl rm -f nginx
	set +x
	if [[ -n ${CHECKS["mount-home"]} ]]; then
		hometmp="$HOME/lima-nerdctl-test-tmp"
		# test for https://github.com/lima-vm/lima/issues/187
		INFO "Testing home bind mount (\"$hometmp\")"
		rm -rf "$hometmp"
		mkdir -p "$hometmp"
		defer "rm -rf \"$hometmp\""
		set -x
		limactl shell "$NAME" nerdctl pull --quiet ${alpine_image}
		echo "random-content-${RANDOM}" >"$hometmp/random"
		expected="$(cat "$hometmp/random")"
		got="$(limactl shell "$NAME" nerdctl run --rm -v "$hometmp/random":/mnt/foo ${alpine_image} cat /mnt/foo)"
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
		limactl shell "$NAME" sudo apt-get install -y netcat
	fi
	if [ "${NAME}" = "fedora" ]; then
		limactl shell "$NAME" sudo dnf install -y nc
	fi
	if [ "${NAME}" = "opensuse" ]; then
		limactl shell "$NAME" sudo zypper in -y netcat-openbsd
	fi
	"${scriptdir}/test-port-forwarding.pl" "${NAME}"

	if [[ -n ${CHECKS["containerd-user"]} || ${NAME} == "alpine" ]]; then
		INFO "Testing that 'nerdctl run' binds to 0.0.0.0 by default and is forwarded to the host"
		if [ "$(uname)" = "Darwin" ]; then
			# macOS runners seem to use `localhost` as the hostname, so the perl lookup just returns `127.0.0.1`
			hostip=$(system_profiler SPNetworkDataType -json | jq -r 'first(.SPNetworkDataType[] | select(.ip_address) | .ip_address) | first')
		else
			hostip=$(perl -MSocket -MSys::Hostname -E 'say inet_ntoa(scalar gethostbyname(hostname()))')
		fi
		if [ -n "${hostip}" ]; then
			sudo=""
			if [ "${NAME}" = "alpine" ]; then
				arch=$(limactl info | jq -r .defaultTemplate.arch)
				nerdctl=$(limactl info | jq -r ".defaultTemplate.containerd.archives[] | select(.arch==\"$arch\").location")
				curl -Lso nerdctl-full.tgz "${nerdctl}"
				limactl shell "$NAME" sudo apk add containerd
				limactl shell "$NAME" sudo rc-service containerd start
				limactl shell "$NAME" sudo tar xzf "${PWD}/nerdctl-full.tgz" -C /usr/local
				rm nerdctl-full.tgz
				sudo="sudo"
			fi
			limactl shell "$NAME" $sudo nerdctl info
			limactl shell "$NAME" $sudo nerdctl pull --quiet ${nginx_image}
			limactl shell "$NAME" $sudo nerdctl run -d --name nginx -p 8888:80 ${nginx_image}

			timeout 3m bash -euxc "until curl -f --retry 30 --retry-connrefused http://${hostip}:8888; do sleep 3; done"
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
	limactl shell "$NAME" sudo apt-get install -y iperf3
	limactl shell "$NAME" iperf3 -s -1 -D
	iperf3 -c "$guestip"
	set +x
	# NOTE: we only test the shared interface here, as the bridged interface cannot be used on GHA (and systemd-networkd-wait-online.service will fail)
fi

if [[ -n ${CHECKS["restart"]} ]]; then
	INFO "Create file in the guest home directory and verify that it still exists after a restart"
	# shellcheck disable=SC2016
	limactl shell "$NAME" sh -c 'touch $HOME/sweet-home'

	INFO "Stopping \"$NAME\""
	limactl stop "$NAME"
	sleep 3

	export ftp_proxy=my.proxy:8021
	INFO "Restarting \"$NAME\""
	limactl start "$NAME"

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
fi

INFO "Stopping \"$NAME\""
limactl stop "$NAME"
sleep 3

INFO "Deleting \"$NAME\""
limactl delete "$NAME"
