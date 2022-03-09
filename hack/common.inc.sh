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
