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

function amazonlinux_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Amazon Linux 2023 image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") <template.yaml>...

Description:
  This script updates the Amazon Linux 2023 image location in the specified templates.
  Image location format:
    https://cdn.amazonlinux.com/al2023/os-images/<version>/<folder>/<filename>

  Latest version information is fetched from:
    https://cdn.amazonlinux.com/al2023/os-images/latest/

Examples:
  Update the Amazon Linux 2023 image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

Flags:
  -h, --help          Print this help message
HELP
}

function amazonlinux_cache_key_for_image_kernel() {
	local location=$1
	case "${location}" in
	https://cdn.amazonlinux.com/al2023/os-images/*) ;;
	*) return 1 ;;
	esac
	# Cache key based on architecture and variant derived from location
	# We use a simple heuristic: file path components.
	# The URL structure is .../os-images/<version>/<folder>/...
	# We want a key that represents "amazonlinux:al2023:<folder>"
	local folder
	if [[ "${location}" == *"/kvm-arm64/"* ]]; then
		folder="kvm-arm64"
	elif [[ "${location}" == *"/kvm/"* ]]; then
		folder="kvm"
	else
		return 1
	fi
	echo "amazonlinux:al2023:${folder}"
}

function amazonlinux_image_entry_for_image_kernel() {
	local location=$1
	case "${location}" in
	https://cdn.amazonlinux.com/al2023/os-images/*) ;;
	*) return 1 ;;
	esac

	local latest_url
	# Get the redirect URL to find the latest version
	latest_url=$(curl -Ls -o /dev/null -w '%{url_effective}' https://cdn.amazonlinux.com/al2023/os-images/latest/)
	local version
	version=$(basename "${latest_url}")

	local arch folder
	if [[ "${location}" == *"x86_64"* ]]; then
		# shellcheck disable=SC2034
		arch="x86_64"
		folder="kvm"
	elif [[ "${location}" == *"arm64"* ]] || [[ "${location}" == *"aarch64"* ]]; then
		# shellcheck disable=SC2034
		arch="aarch64"
		folder="kvm-arm64"
	else
		error_exit "Unknown arch for amazonlinux location: ${location}"
	fi

	local base_url="https://cdn.amazonlinux.com/al2023/os-images/${version}/${folder}"
	local checksums
	checksums=$(download_to_cache "${base_url}/SHA256SUMS")

	local filename digest line
	# Find the qcow2 file in SHA256SUMS
	line=$(grep "\.qcow2$" "${checksums}" | head -n 1)
	[[ -n ${line} ]] || error_exit "No qcow2 image found in ${base_url}/SHA256SUMS"

	# shellcheck disable=SC2034
	digest=$(echo "${line}" | awk '{print "sha256:"$1}')
	filename=$(echo "${line}" | awk '{print $2}')

	# shellcheck disable=SC2034
	local location="${base_url}/${filename}"

	json_vars location arch digest
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
		SUPPORTED_DISTRIBUTIONS+=("amazonlinux")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("amazonlinux")
	fi
	return 0
fi

declare -a templates=()
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		amazonlinux_print_help
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
	amazonlinux_print_help
	exit 0
fi

declare -A image_entry_cache=()

for template in "${templates[@]}"; do
	echo "Processing ${template}"
	# 1. extract location by parsing template
	yq_filter="
		.images[] | [.location, .kernel.location, .kernel.cmdline] | @tsv
	"
	parsed=$(yq eval "${yq_filter}" "${template}")

	# 2. get the image location
	arr=()
	while IFS= read -r line; do arr+=("${line}"); done <<<"${parsed}"
	locations=("${arr[@]}")
	for ((index = 0; index < ${#locations[@]}; index++)); do
		[[ ${locations[index]} != "null" ]] || continue
		set -e
		IFS=$'\t' read -r location _kernel_location _kernel_cmdline <<<"${locations[index]}"
		set +e # Disable 'set -e' to avoid exiting on error for the next assignment.
		cache_key=$(
			set -e # Enable 'set -e' for the next command.
			amazonlinux_cache_key_for_image_kernel "${location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				amazonlinux_image_entry_for_image_kernel "${location}"
			fi
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		set -e
		image_entry_cache[${cache_key}]="${image_entry}"
		if [[ -n ${image_entry} ]]; then
			echo "${image_entry}" | jq
			limactl edit --log-level error --set "
				.images[${index}] = ${image_entry}|
				(.images[${index}] | ..) style = \"double\"
			" "${template}"
		fi
	done
done
