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
hometmp="${HOME_HOST:-$HOME}/lima-test-tmp"
hometmpguest="${HOME_GUEST:-$HOME}/lima-test-tmp"
INFO "Testing home access (\"$hometmp\")"
rm -rf "$hometmp"
mkdir -p "$hometmp"
defer "rm -rf \"$hometmp\""
echo "random-content-${RANDOM}" >"$hometmp/random"
expected="$(cat "$hometmp/random")"
got="$(limactl shell "$NAME" cat "$hometmpguest/random")"
INFO "$hometmp/random: expected=${expected}, got=${got}"
if [ "$got" != "$expected" ]; then
	ERROR "Home directory is not shared?"
	exit 1
fi
