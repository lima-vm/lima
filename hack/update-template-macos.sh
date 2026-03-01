#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function macos_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the macOS image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") <template.yaml>...

Description:
  This script updates the macOS image location in the specified templates.
  Image location format:

    https://updates.cdn-apple.com/.../UniversalMac_<version>_<build>_Restore.ipsw

  Published macOS image information (URL and SHA256 digest) is fetched from
  the ipsw.me API:

    https://api.ipsw.me/v4/device/VirtualMac2,1

  The downloaded JSON will be cached in the Lima cache directory.

Examples:
  Update the macOS image location in templates/_images/macos-*.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/_images/macos-*.yaml

Flags:
  -h, --help           Print this help message
HELP
}

# URL of the ipsw.me device API endpoint for Apple Virtual Machine 1 (VirtualMac2,1).
# This returns all available macOS IPSW firmwares with URL and SHA256 digest.
readonly macos_ipsw_me_device_url="https://api.ipsw.me/v4/device/VirtualMac2,1"

# macos_url_spec_from_location prints the URL spec for the given location.
# If the location is not a macOS IPSW URL from Apple's CDN, it returns 1.
# e.g.
# ```console
# macos_url_spec_from_location https://updates.cdn-apple.com/2025SummerFCS/fullrestores/082-08674/51294E4D-A273-44BE-A280-A69FC347FB87/UniversalMac_15.6_24G84_Restore.ipsw
# {"version":"15.6","major_version":"15","build":"24G84"}
# macos_url_spec_from_location https://updates.cdn-apple.com/2025SummerFCS/fullrestores/093-10809/CFD6DD38-DAF0-40DA-854F-31AAD1294C6F/UniversalMac_15.6.1_24G90_Restore.ipsw
# {"version":"15.6.1","major_version":"15","build":"24G90"}
# ```
function macos_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
		^https://updates\\.cdn-apple\\.com/[^/]+/fullrestores/[^/]+/[^/]+/
		UniversalMac_(?<version>(?<major_version>\\d+)(?:\\.\\d+)+)_(?<build>[^_]+)_Restore\\.ipsw$
	";"x")
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

# macos_latest_image_entry_for_url_spec prints the latest image entry for the given URL spec.
# e.g.
# ```console
# macos_latest_image_entry_for_url_spec '{"major_version":"15"}'
# {"location":"https://updates.cdn-apple.com/.../UniversalMac_15.6.1_24G90_Restore.ipsw","arch":"aarch64","digest":"sha256:..."}
# ```
function macos_latest_image_entry_for_url_spec() {
	local url_spec=$1 major_version ipsw_me_file latest_entry location digest arch="aarch64"
	major_version=$(jq -r '.major_version' <<<"${url_spec}")
	ipsw_me_file=$(download_to_cache "${macos_ipsw_me_device_url}")
	latest_entry=$(jq -e -r --arg major "${major_version}" '
		.firmwares |
		[.[] | select(.version | test("^" + $major + "\\."))] |
		sort_by(.releasedate) |
		last
	' "${ipsw_me_file}")
	[[ -n ${latest_entry} && ${latest_entry} != "null" ]] ||
		error_exit "Failed to get the latest macOS ${major_version} image from ${macos_ipsw_me_device_url}"
	location=$(jq -r '.url' <<<"${latest_entry}")
	digest=$(jq -r '.sha256sum // "" | if . != "" then "sha256:" + . else "" end' <<<"${latest_entry}")
	json_vars location arch digest
}

function macos_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(macos_url_spec_from_location "${location}")
	jq -r '["macos", .major_version] | join(":")' <<<"${url_spec}"
}

function macos_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on macOS" >&2
	url_spec=$(macos_url_spec_from_location "${location}")
	image_entry=$(macos_latest_image_entry_for_url_spec "${url_spec}")
	# shellcheck disable=SC2031
	if [[ -z ${image_entry} ]]; then
		error_exit "Failed to get the image location for ${location}"
	elif jq -e ".location == \"${location}\"" <<<"${image_entry}" >/dev/null; then
		echo "Image location is up-to-date: ${location}" >&2
	else
		echo "${image_entry}"
	fi
}

# check if the script is executed or sourced
# shellcheck disable=SC1091
if [[ ${BASH_SOURCE[0]} == "${0}" ]]; then
	scriptdir=$(dirname "${BASH_SOURCE[0]}")
	# shellcheck source=./cache-common-inc.sh
	. "${scriptdir}/cache-common-inc.sh"

	# shellcheck source=/dev/null # avoid shellcheck hangs on source looping
	. "${scriptdir}/update-template.sh"
else
	# this script is sourced
	if [[ -v SUPPORTED_DISTRIBUTIONS ]]; then
		SUPPORTED_DISTRIBUTIONS+=("macos")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("macos")
	fi
	return 0
fi

declare -a templates=()
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		macos_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	*.yaml) templates+=("$1") ;;
	*)
		error_exit "Unknown argument: $1"
		;;
	esac
	shift
done

if [[ ${#templates[@]} -eq 0 ]]; then
	macos_print_help
	exit 0
fi

declare -A image_entry_cache=()

for template in "${templates[@]}"; do
	echo "Processing ${template}"
	# 1. extract location by parsing template using arch
	yq_filter="
		.images[] | [.location, .kernel.location, .kernel.cmdline] | @tsv
	"
	parsed=$(yq eval "${yq_filter}" "${template}")

	# 3. get the image location
	arr=()
	while IFS= read -r line; do arr+=("${line}"); done <<<"${parsed}"
	locations=("${arr[@]}")
	for ((index = 0; index < ${#locations[@]}; index++)); do
		[[ ${locations[index]} != "null" ]] || continue
		set -e
		IFS=$'\t' read -r location kernel_location kernel_cmdline <<<"${locations[index]}"
		set +e # Disable 'set -e' to avoid exiting on error for the next assignment.
		cache_key=$(
			set -e # Enable 'set -e' for the next command.
			macos_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				macos_image_entry_for_image_kernel "${location}" "${kernel_location}"
			fi
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		set -e
		image_entry_cache[${cache_key}]="${image_entry}"
		if [[ -n ${image_entry} ]]; then
			[[ ${kernel_cmdline} != "null" ]] &&
				jq -e 'has("kernel")' <<<"${image_entry}" >/dev/null &&
				image_entry=$(jq ".kernel.cmdline = \"${kernel_cmdline}\"" <<<"${image_entry}")
			echo "${image_entry}" | jq
			limactl edit --log-level error --set "
				.images[${index}] = ${image_entry}|
				(.images[${index}] | ..) style = \"double\"
			" "${template}"
		fi
	done
done
