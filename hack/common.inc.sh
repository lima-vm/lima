# shellcheck shell=bash
cleanup_cmd=""
trap 'eval ${cleanup_cmd}' EXIT
function defer {
	[ -n "${cleanup_cmd}" ] && cleanup_cmd="${cleanup_cmd}; "
	cleanup_cmd="${cleanup_cmd}$1"
}

function INFO() {
	echo "TEST| [INFO] $*"
}

function WARNING() {
	echo >&2 "TEST| [WARNING] $*"
}

function ERROR() {
	echo >&2 "TEST| [ERROR] $*"
}

if [[ ${BASH_VERSINFO:-0} -lt 4 ]]; then
	ERROR "Bash version is too old: ${BASH_VERSION}"
	exit 1
fi

: "${LIMA_HOME:=${HOME_HOST:-$HOME}/.lima}"
_IPERF3=iperf3
# iperf3-darwin does some magic on macOS to avoid "No route on host" on macOS 15
# https://github.com/lima-vm/socket_vmnet/issues/85
[ "$(uname -s)" = "Darwin" ] && _IPERF3="iperf3-darwin"
: "${IPERF3:=$_IPERF3}"

# Setup LIMA_TEMPLATES_PATH because the templates are not installed, but reference base templates
# via template://_images/* and template://_default/*.
templates_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../templates" && pwd)"
: "${LIMA_TEMPLATES_PATH:-$templates_dir}"
export LIMA_TEMPLATES_PATH
