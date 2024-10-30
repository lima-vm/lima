#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function almalinux_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the AlmaLinux image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--version-major <major version>] <template.yaml>...

Description:
  This script updates the AlmaLinux image location in the specified templates.
  If the image location in the template contains a minor version and release date in the URL,
  the script replaces it with the latest available minor version and date.

  Image location basename format:

	AlmaLinux-<major version>-GenericCloud-[latest|<major version>.<minor version>-<date>].<arch>.qcow2

  Published AlmaLinux image information is fetched from the following URLs:

    https://repo.almalinux.org/almalinux/<major version>/cloud/<arch>/images/

  To parsing html, this script requires 'htmlq' or 'pup' command.      
  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the AlmaLinux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the AlmaLinux image location in ~/.lima/almalinux/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") ~/.lima/almalinux/lima.yaml
  $ limactl factory-reset almalinux

  Update the AlmaLinux image location to major version 9 in ~/.lima/almalinux/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version-major 9 ~/.lima/almalinux/lima.yaml
  $ limactl factory-reset almalinux

Flags:
  --version-major <version>     Use the specified version. The version must be 8 or later.
  -h, --help              Print this help message
HELP
}

# print the URL spec for the given location
function almalinux_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture(
		"^https://repo\\.almalinux\\.org/almalinux/(?<path_version>\\d+(\\.\\d+)?)/cloud/(?<path_arch>[^/]+)/images/" +
		"AlmaLinux-(?<major_version>\\d+)-(?<target_vendor>.*)-" +
		"(latest|(?<major_minor_version>\\d+\\.\\d+)-(?<date>\\d{8}))\\.(?<arch>[^.]+).(?<file_extension>.*)$"
	;"x")
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")

	jq -e '.path_arch == .arch' <<<"${url_spec}" >/dev/null ||
		error_exit "Validation failed: .path_arch != .arch: ${location}"
	echo "${url_spec}"
}

readonly almalinux_jq_filter_directory='"https://repo.almalinux.org/almalinux/\(.path_version)/cloud/\(.path_arch)/images/"'
readonly almalinux_jq_filter_filename='"AlmaLinux-\(.major_version)-\(.target_vendor)-\(if .date then .major_minor_version + "-" + .date else "latest" end).\(.arch).\(.file_extension)"'

# print the location for the given URL spec
function almalinux_location_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${almalinux_jq_filter_directory} + ${almalinux_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the location for ${url_spec}"
}

function almalinux_image_directory_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${almalinux_jq_filter_directory}" <<<"${url_spec}" ||
		error_exit "Failed to get the image directory for ${url_spec}"
}

function almalinux_image_filename_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${almalinux_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the image filename for ${url_spec}"
}

#
function almalinux_latest_image_entry_for_url_spec() {
	local url_spec=$1 arch major_version_url_spec major_version_image_directory downloaded_page links_in_page latest_minor_version_info
	arch=$(jq -r '.arch' <<<"${url_spec}")
	# to detect minor version updates, we need to get the major version URL
	major_version_url_spec=$(jq -e -r '.path_version = .major_version' <<<"${url_spec}")
	major_version_image_directory=$(almalinux_image_directory_from_url_spec "${major_version_url_spec}")
	downloaded_page=$(download_to_cache "${major_version_image_directory}")
	if command -v htmlq >/dev/null; then
		links_in_page=$(htmlq 'pre a' --attribute href <"${downloaded_page}")
	elif command -v pup >/dev/null; then
		links_in_page=$(pup 'pre a attr{href}' <"${downloaded_page}")
	else
		error_exit "Please install 'htmlq' or 'pup' to list images from ${major_version_image_directory}"
	fi
	latest_minor_version_info=$(jq -e -Rrs --argjson spec "${url_spec}" '
		[
			split("\n").[] |
			capture(
				"^AlmaLinux-\($spec.major_version)-\($spec.target_vendor)-" +
				"(?<major_minor_version>\($spec.major_version)\\.\\d+)-" +
				"(?<date>\\d{8})\\.\($spec.arch)\\.\($spec.file_extension)$"
				;"x"
			) |
			.version_number_array = ([.major_minor_version | scan("\\d+") | tonumber])
		] | sort_by(.version_number_array, .date_and_ci_job_id) | last
	' <<<"${links_in_page}")
	[[ -n ${latest_minor_version_info} ]] || return
	local newer_url_spec location directory checksum_location downloaded_sha256sum filename digest
	# prefer the major_minor_version in the path
	newer_url_spec=$(jq -e -r ". + ${latest_minor_version_info} | .path_version = .major_minor_version" <<<"${url_spec}")
	location=$(almalinux_location_from_url_spec "${newer_url_spec}")
	directory=$(almalinux_image_directory_from_url_spec "${newer_url_spec}")
	checksum_location="${directory}CHECKSUM"
	downloaded_sha256sum=$(download_to_cache "${checksum_location}")
	filename=$(almalinux_image_filename_from_url_spec "${newer_url_spec}")
	digest=$(awk "/${filename}/{print \"sha256:\"\$1}" "${downloaded_sha256sum}")
	[[ -n ${digest} ]] || error_exit "Failed to get the SHA256 digest for ${filename}"
	json_vars location arch digest
}

function almalinux_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(almalinux_url_spec_from_location "${location}")
	jq -r '["almalinux", .major_minor_version // .major_version, .target_vendor,
		if .date then "timestamped" else "latest" end,
		.arch, .file_extension] | join(":")' <<<"${url_spec}"
}

function almalinux_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 overriding=${3:-"{}"} url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on AlmaLinux" >&2
	url_spec=$(almalinux_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	if jq -e '.date' <<<"${url_spec}" >/dev/null; then
		image_entry=$(almalinux_latest_image_entry_for_url_spec "${url_spec}")
	else
		image_entry=$(
			# shellcheck disable=SC2030
			location=$(almalinux_location_from_url_spec "${url_spec}")
			location=$(validate_url_without_redirect "${location}")
			arch=$(jq -r '.path_arch' <<<"${url_spec}")
			json_vars location arch
		)
	fi
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

	if ! command -v htmlq >/dev/null && ! command -v pup >/dev/null; then
		error_exit "Please install 'htmlq' or 'pup' to list images from https://repo.almalinux.org/almalinux/<version>/cloud/<arch>/images/"
	fi
	# shellcheck source=/dev/null # avoid shellcheck hangs on source looping
	. "${scriptdir}/update-template.sh"
else
	# this script is sourced
	if ! command -v htmlq >/dev/null && ! command -v pup >/dev/null; then
		echo "Please install 'htmlq' or 'pup' to list images from https://repo.almalinux.org/almalinux/<version>/cloud/<arch>/images/" >&2
	elif [[ -v SUPPORTED_DISTRIBUTIONS ]]; then
		SUPPORTED_DISTRIBUTIONS+=("almalinux")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("almalinux")
	fi
	return 0
fi

declare -a templates=()
declare overriding="{}"
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		almalinux_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	--version-major)
		if [[ -n $2 && $2 != -* ]]; then
			overriding=$(
				major_version="${2%%.*}"
				[[ ${major_version} -ge 8 ]] || error_exit "AlmaLinux major version must be 8 or later"
				# shellcheck disable=2034
				path_version="${major_version}"
				json_vars path_version major_version <<<"${overriding}"
			)
			shift
		else
			error_exit "--version-major requires a value"
		fi
		;;
	--version-major=*)
		overriding=$(
			major_version="${1#*=}"
			major_version="${major_version%%.*}"
			[[ ${major_version} -ge 8 ]] || error_exit "AlmaLinux major version must be 8 or later"
			# shellcheck disable=2034
			path_version="${major_version}"
			json_vars path_version major_version <<<"${overriding}"
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
	almalinux_print_help
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
			almalinux_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				almalinux_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
