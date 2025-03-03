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

function opensuse_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the openSUSE Linux image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--version-major-minor (<major>.<minor>|current|stable|tumbleweed)|--version-major <major> --version-minor <minor>] <template.yaml>...

Description:
  This script updates the openSUSE Linux image location in the specified templates.
  Image location basename format:

    openSUSE-(Leap-<major minor version>|Tumbleweed)-Minimal-VM.<arch>-Cloud.qcow2

  Published openSUSE Linux image information is fetched from the following URLs:

    Leap:
	  <major>.<minor>: https://download.opensuse.org/distribution/leap/<major>.<minor>/appliances/?jsontable
	  current: https://download.opensuse.org/distribution/openSUSE-current/appliances/?jsontable
	  stable: https://download.opensuse.org/distribution/openSUSE-stable/appliances/?jsontable
	
    Tumbleweed:
      x86_64: https://download.opensuse.org/tumbleweed/appliances/?jsontable
	  not x86_64: https://download.opensuse.org/ports/<arch>/tumbleweed/appliances/?jsontable

  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the openSUSE Linux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the openSUSE Linux image location to version 15.6 in ~/.lima/opensuse/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version-major-minor 15.6 ~/.lima/opensuse/lima.yaml
  $ limactl factory-reset opensuse

  Update the openSUSE Linux image location to tumbleweed in ~/.lima/opensuse/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version-major-minor tumbleweed ~/.lima/opensuse/lima.yaml
  $ limactl factory-reset opensuse

Flags:
  --version-major-minor (<major>.<minor>|current|stable|tumbleweed) Use the specified <major>.<minor> version or 
                                                                    aliases "current", "stable", or "tumbleweed".
                                                                    The <major>.<minor> version must be 15.0 or later.
  --version-major <major> --version-minor <minor>                   Use the specified <major> and <minor> version.
  -h, --help                                                        Print this help message
HELP
}

# print the URL spec for the given location
function opensuse_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
		^https://download\\.opensuse\\.org/(?:
			distribution/(?:
				leap/(?<path_version_leap>\\d+\\.\\d+)|
				openSUSE-(?<path_version_leap_alias>current|stable)
			)|
			(?:ports/aarch64/)?(?<path_version_tumbleweed>tumbleweed)
		)/appliances/
		openSUSE-(?<version>Leap-\\d+\\.\\d+|Tumbleweed)-Minimal-VM
		\\.(?<arch>[^-]+)(?<major_minor_patch>-\\d+\\.\\d+\\.\\d+)?-(?<target_vendor>.*)(?<build_info>-Build\\d+\\.\\d+)?\\.(?<file_extension>.*)$
	";"x") | 
	.path_version = (.path_version_leap // .path_version_leap_alias // .path_version_tumbleweed)
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

readonly opensuse_jq_filter_directory='"https://download.opensuse.org/\(
	if .path_version == "tumbleweed" then
		if .arch != "x86_64" then
			"ports/\(.arch)/"
		else
			""
		end + "tumbleweed"
	else
		"distribution/" +
		if .path_version == "current" or .path_version == "stable" then
			"openSUSE-\(.path_version)"
		else
			"leap/\(.path_version)"
		end
	end
)/appliances/"'
readonly opensuse_jq_filter_filename='
	"openSUSE-\(
		if .version == "tumbleweed" then "Tumbleweed" else "Leap-\(.version)" end
	)-Minimal-VM.\(.arch)\(
		if .major_minor_patch then .major_minor_patch else "" end
	)-\(.target_vendor)\(
		if .build_info then .build_info else "" end
	).\(.file_extension)"
'

# print the location for the given URL spec
function opensuse_location_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${opensuse_jq_filter_directory} + ${opensuse_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the location for ${url_spec}"
}

function opensuse_image_directory_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${opensuse_jq_filter_directory}" <<<"${url_spec}" ||
		error_exit "Failed to get the image directory for ${url_spec}"
}

function opensuse_image_filename_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${opensuse_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the image filename for ${url_spec}"
}

function opensuse_json_url_from_url_spec() {
	local -r url_spec=$1
	local json_url
	json_url="$(opensuse_image_directory_from_url_spec "${url_spec}")?jsontable"
	echo "${json_url}"
}

#
function opensuse_latest_image_entry_for_url_spec() {
	local url_spec=$1 arch json_url downloaded_json latest_version_info
	# shellcheck disable=SC2034
	arch=$(jq -r '.arch' <<<"${url_spec}")
	json_url="$(opensuse_image_directory_from_url_spec "${url_spec}")?jsontable"
	downloaded_json=$(download_to_cache "${json_url}")
	latest_version_info=$(jq -e -r --argjson spec "${url_spec}" '
		[
			.data |sort_by(.mtime)|.[].name|
			if $spec.major_minor_patch then
				capture(
					"^openSUSE-(?:Leap-(?<version_leap>\\d+\\.\\d+)|(?<version_tumbleweed>Tumbleweed))-Minimal-VM
					\\.\($spec.arch)(?<major_minor_patch>-\\d+\\.\\d+\\.\\d+)?-\($spec.target_vendor)(?<build_info>-Build\\d+\\.\\d+)?\\.\($spec.file_extension)$"
					;"x"
				)
			else
				capture(
					"^openSUSE-(?:Leap-(?<version_leap>\\d+\\.\\d+)|(?<version_tumbleweed>Tumbleweed))-Minimal-VM
					\\.\($spec.arch)-\($spec.target_vendor)\\.\($spec.file_extension)$"
					;"x"
				)
			end |
			.version = (.version_leap // (.version_tumbleweed|ascii_downcase)) |
			.version_number_array = ([.version | scan("\\d+") | tonumber])
		] | sort_by(.version_number_array, .image_revision) | last
	' <"${downloaded_json}")
	[[ -n ${latest_version_info} ]] || return
	local newer_url_spec location
	# prefer the <major>.<minor> in the path
	newer_url_spec=$(jq -e -r ". + ${latest_version_info} | .path_version = .version" <<<"${url_spec}")
	location=$(opensuse_location_from_url_spec "${newer_url_spec}")
	location=$(validate_url_without_redirect "${location}")

	# Digest is not used here because URLs containing dates are not retained long-term.
	# Instead, URLs without dates must be used, and their content is often updated without a URL change,
	# resulting in only the digest being updated. Therefore, recording the digest is not meaningful.
	#
	# local sha256sum_location downloaded_sha256sum filename digest
	# sha256sum_location="${location}.sha256"
	# downloaded_sha256sum=$(download_to_cache "${sha256sum_location}")
	# filename=$(opensuse_image_filename_from_url_spec "${newer_url_spec}")
	# digest="sha256:$(awk '{print $1}' <"${downloaded_sha256sum}")"
	# [[ -n ${digest} ]] || error_exit "Failed to get the digest for ${filename}"
	json_vars location arch # digest
}

function opensuse_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(opensuse_url_spec_from_location "${location}")
	jq -r '["opensuse", .path_version, .target_vendor, .arch, .file_extension] | join(":")' <<<"${url_spec}"
}

function opensuse_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 url_spec path_version overriding image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on openSUSE Linux" >&2
	url_spec=$(opensuse_url_spec_from_location "${location}")
	path_version=$(jq -r '.path_version' <<<"${url_spec}")
	if [[ ${path_version} == "tumbleweed" ]]; then
		overriding=${3:-'{"path_version":"tumbleweed"}'}
	else
		overriding=${3:-'{"path_version":"stable"}'}
	fi
	url_spec=$(jq -r '. + '"${overriding}" <<<"${url_spec}")
	image_entry=$(opensuse_latest_image_entry_for_url_spec "${url_spec}")
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
		SUPPORTED_DISTRIBUTIONS+=("opensuse")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("opensuse")
	fi
	return 0
fi

declare -a templates=()
declare overriding='{}'
declare version_major='' version_minor=''
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		opensuse_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	--version-major-minor)
		if [[ -n ${2:-} && $2 != -* ]]; then
			version="$2"
			shift
		else
			error_exit "--version-major-minor requires a value"
		fi
		;&
	--version-major-minor=*)
		version=${version:-${1#*=}}
		overriding=$(
			version="${version#v}"
			if [[ ${version} =~ ^v?[0-9]+.[0-9]+ ]]; then
				version="$(echo "${version}" | cut -d. -f1-2)"
				[[ ${version%%.*} -ge 15 ]] || error_exit "openSUSE Linux version must be 15.0 or later"
				path_version="${version}"
			elif [[ ${version} == "current" || ${version} == "stable" || ${version} == "tumbleweed" ]]; then
				path_version=${version}
			else
				error_exit "--version-major-minor requires a value in the format <major>.<minor>, current, stable, or tumbleweed"
			fi
			json_vars path_version <<<"${overriding}"
		)
		;;
	--version-major)
		if [[ -n ${2:-} && $2 != -* ]]; then
			version_major="$2"
			shift
		else
			error_exit "--version-major requires a value"
		fi
		;&
	--version-major=*)
		version_major=${version_major:-${1#*=}}
		[[ ${version_major} =~ ^[0-9]+$ ]] || error_exit "Please specify --version-major in numbers"
		;;
	--version-minor)
		if [[ -n ${2:-} && $2 != -* ]]; then
			version_minor="$2"
			shift
		else
			error_exit "--version-minor requires a value"
		fi
		;&
	--version-minor=*)
		version_minor=${version_minor:-${1#*=}}
		[[ ${version_minor} =~ ^[0-9]+$ ]] || error_exit "Please specify --version-minor in numbers"
		;;
	*.yaml) templates+=("$1") ;;
	*)
		error_exit "Unknown argument: $1"
		;;
	esac
	shift
	[[ -z ${overriding} ]] && overriding="{}"
done

if ! jq -e '.path_version' <<<"${overriding}" >/dev/null; then # --version-major-minor is not specified
	if [[ -n ${version_major} && -n ${version_minor} ]]; then
		[[ ${version_major} -ge 15 ]] || error_exit "openSUSE Linux version must be 15.0 or later"
		# shellcheck disable=2034
		path_version="${version_major}.${version_minor}"
		overriding=$(json_vars path_version <<<"${overriding}")
	elif [[ -n ${version_major} ]]; then
		error_exit "--version-minor is required when --version-major is specified"
	elif [[ -n ${version_minor} ]]; then
		error_exit "--version-major is required when --version-minor is specified"
	fi
elif [[ -n ${version_major} || -n ${version_minor} ]]; then # --version-major-minor is specified
	echo "Ignoring --version-major and --version-minor because --version-major-minor is specified" >&2
fi
[[ ${overriding} == "{}" ]] && overriding=''

if [[ ${#templates[@]} -eq 0 ]]; then
	opensuse_print_help
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
			opensuse_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				opensuse_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
