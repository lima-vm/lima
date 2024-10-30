#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function alpine_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Alpine Linux image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--version-major-minor (<major>.<minor>|latest-stable)] <template.yaml>...

Description:
  This script updates the Alpine Linux image location in the specified templates.
  Image location basename format:

    <target vendor>_alpine-<version>-<arch>-<firmware>-<bootstrap>[-<machine>]-<image revision>.qcow2

  Published Alpine Linux image information is fetched from the following URLs:

    latest-stable: https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/cloud
    <major>.<minor>: https://dl-cdn.alpinelinux.org/alpine/v<major>.<minor>/releases/cloud

  To parsing html, this script requires 'htmlq' or 'pup' command.      
  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the Alpine Linux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Alpine Linux image location to version 3.18 in ~/.lima/alpine/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version-major-minor 3.18 ~/.lima/alpine/lima.yaml
  $ limactl factory-reset alpine

Flags:
  --version-major-minor (<major>.<minor>|latest-stable)  Use the specified <major>.<minor> version or alias "latest-stable".
                                                         The <major>.<minor> version must be 3.18 or later.
  -h, --help                                             Print this help message
HELP
}

# print the URL spec for the given location
function alpine_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
		^https://dl-cdn\\.alpinelinux\\.org/alpine/(?<path_version>v\\d+\\.\\d+|latest-stable)/releases/cloud/
		(?<target_vendor>[^_]+)_alpine-(?<version>\\d+\\.\\d+\\.\\d+)-(?<arch>[^-]+)-
		(?<firmware>[^-]+)-(?<bootstrap>[^-]+)(-(?<machine>metal|vm))?-(?<image_revision>r\\d+)\\.(?<file_extension>.*)$
	";"x")
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

readonly alpine_jq_filter_directory='"https://dl-cdn.alpinelinux.org/alpine/\(.path_version)/releases/cloud/"'
readonly alpine_jq_filter_filename='
	"\(.target_vendor)_alpine-\(.version)-\(.arch)-\(.firmware)-\(.bootstrap)" +
	"\(if .machine then "-" + .machine else "" end)-\(.image_revision).\(.file_extension)"
'

# print the location for the given URL spec
function alpine_location_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${alpine_jq_filter_directory} + ${alpine_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the location for ${url_spec}"
}

function alpine_image_directory_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${alpine_jq_filter_directory}" <<<"${url_spec}" ||
		error_exit "Failed to get the image directory for ${url_spec}"
}

function alpine_image_filename_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${alpine_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the image filename for ${url_spec}"
}

#
function alpine_latest_image_entry_for_url_spec() {
	local url_spec=$1 arch image_directory downloaded_page links_in_page latest_version_info
	# shellcheck disable=SC2034
	arch=$(jq -r '.arch' <<<"${url_spec}")
	image_directory=$(alpine_image_directory_from_url_spec "${url_spec}")
	downloaded_page=$(download_to_cache "${image_directory}")
	if command -v htmlq >/dev/null; then
		links_in_page=$(htmlq 'pre a' --attribute href <"${downloaded_page}")
	elif command -v pup >/dev/null; then
		links_in_page=$(pup 'pre a attr{href}' <"${downloaded_page}")
	else
		error_exit "Please install 'htmlq' or 'pup' to list images from ${image_directory}"
	fi
	latest_version_info=$(jq -e -Rrs --argjson spec "${url_spec}" '
		[
			split("\n").[] |
			capture(
				"^\($spec.target_vendor)_alpine-(?<version>\\d+\\.\\d+\\.\\d+)-\($spec.arch)-" +
				"\($spec.firmware)-\($spec.bootstrap)\(if $spec.machine then "-" + $spec.machine else "" end)-" +
				"(?<image_revision>r\\d+)\\.\($spec.file_extension)"
				;"x"
			) |
			.version_number_array = ([.version | scan("\\d+") | tonumber])
		] | sort_by(.version_number_array, .image_revision) | last
	' <<<"${links_in_page}")
	[[ -n ${latest_version_info} ]] || return
	local newer_url_spec location sha512sum_location downloaded_sha256sum filename digest
	# prefer the v<major>.<minor> in the path
	newer_url_spec=$(jq -e -r ". + ${latest_version_info} | .path_version = \"v\" + (.version_number_array[:2]|map(tostring)|join(\".\"))" <<<"${url_spec}")
	location=$(alpine_location_from_url_spec "${newer_url_spec}")
	location=$(validate_url_without_redirect "${location}")
	sha512sum_location="${location}.sha512"
	downloaded_sha256sum=$(download_to_cache "${sha512sum_location}")
	filename=$(alpine_image_filename_from_url_spec "${newer_url_spec}")
	digest="sha512:$(<"${downloaded_sha256sum}")"
	[[ -n ${digest} ]] || error_exit "Failed to get the digest for ${filename}"
	json_vars location arch digest
}

function alpine_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(alpine_url_spec_from_location "${location}")
	jq -r '["alpine", .path_version, .target_vendor, .arch, .file_extension] | join(":")' <<<"${url_spec}"
}

function alpine_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 overriding=${3:-"{}"} url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on Alpine Linux" >&2
	url_spec=$(alpine_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	image_entry=$(alpine_latest_image_entry_for_url_spec "${url_spec}")
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
		error_exit "Please install 'htmlq' or 'pup' to list images from https://dl-cdn.alpinelinux.org/alpine/<version>/releases/cloud/"
	fi
	# shellcheck source=/dev/null # avoid shellcheck hangs on source looping
	. "${scriptdir}/update-template.sh"
else
	# this script is sourced
	if ! command -v htmlq >/dev/null && ! command -v pup >/dev/null; then
		echo "Please install 'htmlq' or 'pup' to list images from https://dl-cdn.alpinelinux.org/alpine/<version>/releases/cloud/" >&2
	elif [[ -v SUPPORTED_DISTRIBUTIONS ]]; then
		SUPPORTED_DISTRIBUTIONS+=("alpine")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("alpine")
	fi
	return 0
fi

declare -a templates=()
declare overriding='{"path_version":"latest-stable"}'
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		alpine_print_help
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
				[[ ${version%%.*} -gt 3 || (${version%%.*} -eq 3 && ${version#*.} -ge 18) ]] || error_exit "Alpine Linux version must be 3.18 or later"
				path_version="v${version}"
			elif [[ ${version} == "latest-stable" ]]; then
				path_version="latest-stable"
			else
				error_exit "--version-major-minor requires a value in the format <major>.<minor> or latest-stable"
			fi
			# shellcheck disable=2034
			json_vars path_version <<<"${overriding}"
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
	alpine_print_help
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
			alpine_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				alpine_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
