#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function fedora_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Fedora Linux image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--version (<version number>|release|development[/<version number>]|rawhide)] <template.yaml>...

Description:
  This script updates the Fedora Linux image location in the specified templates.
  Image location basename format:

    Fedora-Cloud-Base[-<target vendor>]-<version>-<build info>.<arch>.qcow2
    Fedora-Cloud-Base[-<target vendor>].<arch>-<version>-<build info>.qcow2

  Published Fedora Linux image information is fetched from the following URL:

    ${fedora_image_list_url}

  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the Fedora Linux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Fedora Linux image location to version 41 in ~/.lima/fedora/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version 41 ~/.lima/fedora/lima.yaml
  $ limactl factory-reset fedora

Flags:
  --version <version> Use the specified version.
                      The version must be <version number>, 'release', 'development[/<version number>]', or 'rawhide'.
  -h, --help          Print this help message
HELP
}

# print the URL spec for the given location
function fedora_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
			^https://download\\.fedoraproject\\.org/pub/fedora/linux/(?<path_version>(releases|development)/(\\d+|rawhide))/Cloud/(?<path_arch>[^/]+)/images/
			Fedora-Cloud-Base(?<target_vendor>-Generic)?(
				(-(?<version_before_arch>\\d+|Rawhide)-(?<build_info_before_arch>[^-]+)(?<arch_postfix>\\.[^.]+))|
				((?<arch_prefix>\\.[^-]+)-(?<version_after_arch>\\d+|Rawhide)-(?<build_info_after_arch>[^-]+))
			)\\.(?<file_extension>.*)$
		";"x") |
		.version = (.version_before_arch // .version_after_arch) |
		.build_info = (.build_info_before_arch // .build_info_after_arch ) |
		map_values(. // empty) # remove null values
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

readonly fedora_jq_filter_directory='"https://download.fedoraproject.org/pub/fedora/linux/\(.path_version)/Cloud/\(.path_arch)/images/"'
readonly fedora_jq_filter_filename='
	"Fedora-Cloud-Base\(.target_vendor // "")\(.arch_prefix // "")-\(.version)-\(.build_info)\(.arch_postfix // "").\(.file_extension)"
'

readonly fedora_jq_filter_checksum_filename='
	"Fedora-Cloud-\(
		if .path_version|startswith("development/") then
			"images-\(.version)-\(.path_arch)-\(.build_info)"
		else
			"\(.version)-\(.build_info)-\(.path_arch)"
		end
	)-CHECKSUM"
'

# print the location for the given URL spec
function fedora_location_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${fedora_jq_filter_directory} + ${fedora_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the location for ${url_spec}"
}

function fedora_image_directory_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${fedora_jq_filter_directory}" <<<"${url_spec}" ||
		error_exit "Failed to get the image directory for ${url_spec}"
}

function fedora_image_filename_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${fedora_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the image filename for ${url_spec}"
}

function fedora_image_checksum_filename_from_url_spec() {
	local -r url_spec=$1
	jq -e -r "${fedora_jq_filter_checksum_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the checksum filename for ${url_spec}"
}

readonly fedora_image_list_url='https://dl.fedoraproject.org/pub/fedora/imagelist-fedora'
#
function fedora_latest_image_entry_for_url_spec() {
	local url_spec=$1 arch image_list spec_for_query latest_version_info
	# shellcheck disable=SC2034
	arch=$(jq -r '.path_arch' <<<"${url_spec}")
	image_list=$(download_to_cache "${fedora_image_list_url}")
	spec_for_query=$(jq -r '. | {path_version, path_arch, file_extension}' <<<"${url_spec}")
	latest_version_info=$(jq -e -Rrs --argjson spec "${spec_for_query}" '
		[
			split("\n").[] |
			capture("
				^\\./linux/(?<path_version>\($spec.path_version))/Cloud/\($spec.path_arch)/images/
				Fedora-Cloud-Base(?<target_vendor>-Generic)?(
					-(?<version_before_arch>\\d+|Rawhide)-(?<build_info_before_arch>[^-]+)(?<arch_postfix>\\.\($spec.path_arch))|
					(?<arch_prefix>\\.\($spec.path_arch))-(?<version_after_arch>\\d+|Rawhide)-(?<build_info_after_arch>[^-]+)
				)\\.\($spec.file_extension)$
			";"x") |
			.version = (.version_before_arch // .version_after_arch) |
			.build_info = (.build_info_before_arch // .build_info_after_arch) |
			# do not remove null values. we need them for creating newer_url_spec
			# map_values(. // empty) |
			.version_number_array = ([(if (.version|test("\\d+")) then (.version|tonumber) else .version end)] + [.build_info | scan("\\d+") | tonumber])
		] | sort_by(.version_number_array) | last
	' <"${image_list}" || error_exit "Failed to get the latest version info for ${spec_for_query}")
	[[ -n ${latest_version_info} ]] || return
	local newer_url_spec directory filename location sha512sum_location downloaded_sha256sum digest
	# prefer the v<major>.<minor> in the path
	newer_url_spec=$(jq -e -r ". + ${latest_version_info}" <<<"${url_spec}")
	directory=$(fedora_image_directory_from_url_spec "${newer_url_spec}")
	filename=$(fedora_image_filename_from_url_spec "${newer_url_spec}")
	location="${directory}${filename}"
	# validate the location. use original url since the location may be redirected to some mirror
	location=$(validate_url_without_redirect "${location}")
	sha512sum_location="${directory}$(fedora_image_checksum_filename_from_url_spec "${newer_url_spec}")"
	# download the checksum file and get the sha256sum
	# cache original url since the checksum file may be redirected to some mirror
	downloaded_sha256sum=$(download_to_cache_without_redirect "${sha512sum_location}")
	digest="sha256:$(awk "/SHA256 \(${filename}\) =/{print \$4}" "${downloaded_sha256sum}")"
	[[ -n ${digest} ]] || error_exit "Failed to get the digest for ${filename}"
	json_vars location arch digest
}

function fedora_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(fedora_url_spec_from_location "${location}")
	jq -r '["fedora", .path_version, .target_vendor, .path_arch, .file_extension] | join(":")' <<<"${url_spec}"
}

function fedora_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 overriding=${3:-'{"path_version":"releases/\\d+"}'} url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on Fedora Linux" >&2
	url_spec=$(fedora_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	image_entry=$(fedora_latest_image_entry_for_url_spec "${url_spec}")
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
		SUPPORTED_DISTRIBUTIONS+=("fedora")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("fedora")
	fi
	return 0
fi

declare -a templates=()
declare overriding='{}'
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		fedora_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	--version)
		if [[ -n ${2:-} && $2 != -* ]]; then
			version="$2"
			shift
		else
			error_exit "--version requires a value"
		fi
		;&
	--version=*)
		version=${version:-${1#*=}}
		overriding=$(
			[[ ${version} =~ ^[0-9]+$ ]] && path_version="releases/${version}"
			[[ ${version} =~ ^releases?$ ]] && path_version="releases/\d+"
			[[ ${version} == "development" ]] && path_version="development/\d+"
			[[ ${version} =~ ^(releases|development)/([0-9]+)$ ]] && path_version="${version}"
			[[ ${version} =~ ^(development/)?rawhide$ ]] && path_version="development/rawhide"
			[[ -n ${path_version:-} ]] ||
				error_exit "The version must be <version number>, 'release', 'development[/<version number>'], or 'rawhide'."
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
	fedora_print_help
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
			fedora_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				fedora_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
