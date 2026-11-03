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

readonly alpine_iso_latest_release_url='https://api.github.com/repos/lima-vm/alpine-lima/releases/latest'

function alpine_iso_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Alpine Lima ISO image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") <template.yaml>...

Description:
  This script updates the Alpine Lima ISO image location in the specified templates
  to the latest release of https://github.com/lima-vm/alpine-lima.
  The edition and the architecture are kept from the template.
  Image location format:

    https://github.com/lima-vm/alpine-lima/releases/download/<tag>/alpine-lima-<edition>-<alpine version>-<arch>.iso

  Published release information is fetched from the following URL:

    ${alpine_iso_latest_release_url}

  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the Alpine Lima ISO image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

Flags:
  -h, --help              Print this help message
HELP
}

# print the URL spec for the given location
function alpine_iso_url_spec_from_location() {
	local location=$1
	local jq_filter
	local url_spec
	jq_filter='capture("
		^https://github\\.com/lima-vm/alpine-lima/releases/download/(?<tag>[^/]+)/
		alpine-lima-(?<edition>[^-]+)-(?<version>\\d+\\.\\d+\\.\\d+)-(?<arch>[^-]+)\\.iso$
	";"x")
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

function alpine_iso_latest_image_entry_for_url_spec() {
	local url_spec=$1
	local edition
	local arch
	local downloaded_release
	local location
	local filename
	local downloaded_sha512sum
	local digest
	edition=$(jq -r '.edition' <<<"${url_spec}")
	arch=$(jq -r '.arch' <<<"${url_spec}")
	downloaded_release=$(download_to_cache "${alpine_iso_latest_release_url}")
	location=$(jq -e -r --argjson spec "${url_spec}" '
		[
			.assets[] |
			select(.name | test("^alpine-lima-\($spec.edition)-\\d+\\.\\d+\\.\\d+-\($spec.arch)\\.iso$")) |
			.browser_download_url
		] | if length == 1 then .[0] else empty end
	' <"${downloaded_release}") ||
		error_exit "Failed to find exactly one alpine-lima-${edition}-<version>-${arch}.iso asset in ${alpine_iso_latest_release_url}"
	location=$(validate_url_without_redirect "${location}")
	filename=$(basename "${location}")
	downloaded_sha512sum=$(download_to_cache_without_redirect "${location}.sha512sum")
	digest=$(awk -v filename="${filename}" '
		NR == 1 && NF == 2 && $1 ~ /^[0-9a-f]+$/ && length($1) == 128 && $2 == filename { print "sha512:" $1 }
	' "${downloaded_sha512sum}")
	[[ -n ${digest} ]] || error_exit "Failed to get the SHA512 digest for ${filename} from ${location}.sha512sum"
	json_vars location arch digest
}

function alpine_iso_cache_key_for_image_kernel() {
	local location=$1
	local url_spec
	url_spec=$(alpine_iso_url_spec_from_location "${location}")
	jq -r '["alpine_iso", .edition, .arch] | join(":")' <<<"${url_spec}"
}

function alpine_iso_image_entry_for_image_kernel() {
	local location=$1
	local kernel_is_not_supported=$2
	local url_spec
	local image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on Alpine Lima" >&2
	url_spec=$(alpine_iso_url_spec_from_location "${location}")
	image_entry=$(alpine_iso_latest_image_entry_for_url_spec "${url_spec}")
	if [[ -z ${image_entry} ]]; then
		error_exit "Failed to get the ${url_spec} image location for ${location}"
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
		SUPPORTED_DISTRIBUTIONS+=("alpine_iso")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("alpine_iso")
	fi
	return 0
fi

declare -a templates=()
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		alpine_iso_print_help
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
	alpine_iso_print_help
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
			alpine_iso_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				alpine_iso_image_entry_for_image_kernel "${location}" "${kernel_location}"
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
				.images[${index}] *= ${image_entry}|
				(.images[${index}] | ..) style = \"double\"
			" "${template}"
		fi
	done
done
