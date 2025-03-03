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

function oraclelinux_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Oracle Linux image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--version-major <major version>] <template.yaml>...

Description:
  This script updates the Oracle Linux image location in the specified templates.
  Image location basename format:

	OL<major version>U<minor version>_<arch>-kvm[-cloud]-b<build number>.qcow2

  Published Oracle Linux image information is fetched from the following URLs:

    OL8:
	  x86_64: https://yum.oracle.com/templates/OracleLinux/ol8-template.json
	  aarch64: https://yum.oracle.com/templates/OracleLinux/ol8_aarch64-cloud-template.json
	  
	OL9:
	  x86_64: https://yum.oracle.com/templates/OracleLinux/ol9-template.json
	  aarch64: https://yum.oracle.com/templates/OracleLinux/ol9_aarch64-cloud-template.json

  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the Oracle Linux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Oracle Linux image location to major version 9 in ~/.lima/oraclelinux/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version-major 9 ~/.lima/oraclelinux/lima.yaml
  $ limactl factory-reset oraclelinux

Flags:
  --version-major <major version>  Use the specified Oracle Linux <major version>.
                                   The major version must be 7+ for x86_64 or 8+ for aarch64.
  -h, --help                       Print this help message
HELP
}

# print the URL spec for the given location
function oraclelinux_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
		^https://yum\\.oracle\\.com/templates/OracleLinux/OL(?<path_major_version>\\d+)/u(?<path_minor_version>\\d+)/(?<path_arch>[^/]+)/
		OL(?<major_version>\\d+)U(?<minor_version>\\d+)_(?<arch>[^-]+)-(?<type>[^-]+)(?<cloud>-cloud)?-b(?<build_number>\\d+)\\.(?<file_extension>.*)$
	";"x")
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

readonly oraclelinux_jq_filter_json_url='
	"https://yum.oracle.com/templates/OracleLinux/" +
	"ol\(.path_major_version)\(if .path_arch != "x86_64" then "_" + .path_arch else "" end)\(.cloud // "")-template.json"
'

function oraclelinux_json_url_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${oraclelinux_jq_filter_json_url}" <<<"${url_spec}" ||
		error_exit "Failed to get the JSON url for ${url_spec}"
}

function oraclelinux_latest_image_entry_for_url_spec() {
	local url_spec=$1 arch json_url downloaded_json latest_version_info
	# shellcheck disable=SC2034
	arch=$(jq -r '.arch' <<<"${url_spec}")
	json_url=$(oraclelinux_json_url_from_url_spec "${url_spec}")
	downloaded_json=$(download_to_cache "${json_url}")
	latest_version_info="$(jq -e -r --argjson spec "${url_spec}" '{
		location: ("https://yum.oracle.com" + .base_url + "/" + .[$spec.type].image),
		sha256: ("sha256:" + .[$spec.type].sha256)
	}' <"${downloaded_json}")"
	[[ -n ${latest_version_info} ]] || return
	local location digest
	# prefer the v<major>.<minor> in the path
	location=$(jq -e -r '.location' <<<"${latest_version_info}")
	location=$(validate_url_without_redirect "${location}")
	# shellcheck disable=SC2034
	digest=$(jq -e -r '.sha256' <<<"${latest_version_info}")
	json_vars location arch digest
}

function oraclelinux_cache_key_for_image_kernel() {
	local location=$1 overriding=${3:-"{}"} url_spec
	url_spec=$(oraclelinux_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	jq -r '["oraclelinux", .path_major_version, .type, .cloud // empty, .arch, .file_extension] | join(":")' <<<"${url_spec}"
}

function oraclelinux_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 overriding=${3:-"{}"} url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on Oracle Linux" >&2
	url_spec=$(oraclelinux_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	image_entry=$(oraclelinux_latest_image_entry_for_url_spec "${url_spec}")
	# shellcheck disable=SC2031
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
		SUPPORTED_DISTRIBUTIONS+=("oraclelinux")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("oraclelinux")
	fi
	return 0
fi

declare -a templates=()
declare overriding='{}'
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		oraclelinux_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	--version-major)
		if [[ -n $2 && $2 != -* ]]; then
			overriding=$(
				path_major_version="${2}"
				[[ ${path_major_version} =~ ^[0-9]+$ ]] || error_exit "Oracle Linux major version must be a number"
				[[ ${path_major_version} -eq 7 ]] && echo 'Oracle Linux major version 7 exists only for x86_64. It may fail for aarch64.' >&2
				[[ ${path_major_version} -gt 7 ]] || error_exit "Oracle Linux major version must be 7+ for x86_64 or 8+ for aarch64"
				json_vars path_major_version <<<"${overriding}"
			)
			shift
		else
			error_exit "--version-major requires a value"
		fi
		;;
	--version-major=*)
		overriding=$(
			path_major_version="${1#*=}"
			[[ ${path_major_version} =~ ^[0-9]+$ ]] || error_exit "Oracle Linux major version must be a number"
			[[ ${path_major_version} -eq 7 ]] && echo 'Oracle Linux major version 7 exists only for x86_64. It may fail for aarch64.' >&2
			[[ ${path_major_version} -gt 7 ]] || error_exit "Oracle Linux major version must be 7+ for x86_64 or 8+ for aarch64"
			json_vars path_major_version <<<"${overriding}"
		)
		;;
	*.yaml) templates+=("$1") ;;
	*)
		error_exit "Unknown argument: $1"
		;;
	esac
	shift
	[[ -z ${overriding} ]] && overriding="{}"
done

if [[ ${#templates[@]} -eq 0 ]]; then
	oraclelinux_print_help
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
			oraclelinux_cache_key_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				oraclelinux_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
