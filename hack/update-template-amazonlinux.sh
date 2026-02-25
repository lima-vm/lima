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
$(basename "${BASH_SOURCE[0]}"): Update the Amazon Linux (AL2023) image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") <template.yaml>...

Description:
  This script updates Amazon Linux 2023 (AL2023) KVM image locations + digests.

  Published Amazon Linux image information is fetched from the following URL:
    https://cdn.amazonlinux.com/al2023/os-images/latest/

Examples:
  Update the Amazon Linux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update ~/.lima/amazonlinux/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") ~/.lima/amazonlinux/lima.yaml
  $ limactl factory-reset amazonlinux

Flags:
  -h, --help           Print this help message
HELP
}

readonly amazonlinux_latest_root_url='https://cdn.amazonlinux.com/al2023/os-images/latest/'

function amazonlinux_latest_release() {
	if [[ -n ${AMAZONLINUX_LATEST_RELEASE:-} ]]; then
		echo "${AMAZONLINUX_LATEST_RELEASE}"
		return
	fi
	local effective
	effective=$(validate_url "${amazonlinux_latest_root_url}")
	[[ ${effective} =~ /al2023/os-images/([^/]+)/?$ ]] || error_exit "Failed to parse release from ${effective}"
	AMAZONLINUX_LATEST_RELEASE="${BASH_REMATCH[1]}"
	echo "${AMAZONLINUX_LATEST_RELEASE}"
}

# print the URL spec for the given location
function amazonlinux_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
		^https://cdn\\.amazonlinux\\.com/al2023/os-images/(?<release>[0-9.]+)/(?<variant>kvm(-arm64)?)/
		al2023-kvm-(?<filename_release>[0-9.]+)-kernel-(?<kernel_version>[0-9.]+)-(?<arch_token>x86_64|arm64)\\.(?<file_extension>.*)$
	";"x")'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	jq -e '.release == .filename_release' <<<"${url_spec}" >/dev/null ||
		error_exit "Validation failed: .release != .filename_release: ${location}"
	echo "${url_spec}"
}

readonly amazonlinux_jq_filter_directory='"https://cdn.amazonlinux.com/al2023/os-images/\(.release)/\(.variant)/"'
readonly amazonlinux_jq_filter_filename='"al2023-kvm-\(.filename_release)-kernel-\(.kernel_version)-\(.arch_token).\(.file_extension)"'

function amazonlinux_image_directory_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${amazonlinux_jq_filter_directory}" <<<"${url_spec}" ||
		error_exit "Failed to get the image directory for ${url_spec}"
}

function amazonlinux_image_filename_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${amazonlinux_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the image filename for ${url_spec}"
}

function amazonlinux_limayaml_arch_from_arch_token() {
	local token=$1
	case "${token}" in
	x86_64) echo x86_64 ;;
	arm64) echo aarch64 ;;
	*) error_exit "Unknown arch token: ${token}" ;;
	esac
}

function amazonlinux_latest_filename_in_directory() {
	local directory_url=$1 release=$2 arch_token=$3 file_extension=$4 downloaded_page links latest_filename
	downloaded_page=$(download_to_cache "${directory_url}")
	# Extract href values from the generated index page.
	links=$(grep -oE 'href="[^"]+"' "${downloaded_page}" | sed -E 's/^href="([^"]+)"$/\1/' || true)
	latest_filename=$(jq -e -Rrs --arg release "${release}" --arg arch "${arch_token}" --arg ext "${file_extension}" '
		[
			split("\n").[] as $f |
			select($f != "" and ($f|endswith("/"))|not) |
			(
				$f |
				capture("^al2023-kvm-(?<release>[0-9.]+)-kernel-(?<kernel_version>[0-9.]+)-(?<arch_token>[^.]+)\\.(?<file_extension>.*)$";"x")
			) as $m |
			$m
			| select(.release == $release and .arch_token == $arch and .file_extension == $ext)
			| .filename = $f
			| .kernel_number_array = ([.kernel_version | scan("\\d+") | tonumber])
		] | sort_by(.kernel_number_array) | last | .filename
	' <<<"${links}")
	[[ -n ${latest_filename} && ${latest_filename} != "null" ]] ||
		error_exit "Failed to find matching AL2023 KVM image in ${directory_url} for ${arch_token}.${file_extension}"
	echo "${latest_filename}"
}

function amazonlinux_latest_image_entry_for_url_spec() {
	local url_spec=$1 arch_token file_extension latest_release newer_url_spec directory filename location sha256sum_location downloaded_sha256sum digest arch
	arch_token=$(jq -r '.arch_token' <<<"${url_spec}")
	file_extension=$(jq -r '.file_extension' <<<"${url_spec}")
	latest_release=$(amazonlinux_latest_release)
	newer_url_spec=$(jq -e -r --arg release "${latest_release}" '.release=$release | .filename_release=$release' <<<"${url_spec}")
	directory=$(amazonlinux_image_directory_from_url_spec "${newer_url_spec}")
	filename=$(amazonlinux_latest_filename_in_directory "${directory}" "${latest_release}" "${arch_token}" "${file_extension}")
	location="${directory}${filename}"
	location=$(validate_url_without_redirect "${location}")
	sha256sum_location="${directory}SHA256SUMS"
	downloaded_sha256sum=$(download_to_cache_without_redirect "${sha256sum_location}")
	digest="sha256:$(awk -v f="${filename}" '$2 == f {print $1}' "${downloaded_sha256sum}")"
	[[ ${digest} != "sha256:" ]] || error_exit "Failed to get the digest for ${filename} from ${sha256sum_location}"
	# shellcheck disable=SC2034
	arch=$(amazonlinux_limayaml_arch_from_arch_token "${arch_token}")
	json_vars location arch digest
}

function amazonlinux_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(amazonlinux_url_spec_from_location "${location}")
	jq -r '["amazonlinux", "al2023", .variant, .arch_token, .file_extension] | join(":")' <<<"${url_spec}"
}

function amazonlinux_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on Amazon Linux" >&2
	url_spec=$(amazonlinux_url_spec_from_location "${location}")
	image_entry=$(amazonlinux_latest_image_entry_for_url_spec "${url_spec}")
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
			amazonlinux_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				amazonlinux_image_entry_for_image_kernel "${location}" "${kernel_location}"
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
