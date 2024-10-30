#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function ubuntu_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Ubuntu image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--flavor <flavor>|--minimal|--server] [--version <version>] <template.yaml>...

Description:
  This script updates the Ubuntu image location in the specified templates.
  If the image location in the template contains a release date in the URL, the script replaces it with the latest available date.
  If no flags are specified, the script uses the flavor and version from the image location basename in the template.

  Image location basename format: ubuntu-<version>-<flavor>-cloudimg-<arch>.img

  Released Ubuntu image information is fetched from the following URLs:

    Server: https://cloud-images.ubuntu.com/releases/stream/v1/com.ubuntu.cloud:released:download.json
    Minimal: https://cloud-images.ubuntu.com/minimal/releases/stream/v1/com.ubuntu.cloud:released:download.json

  The downloaded JSON file will be cached in the Lima cache directory.

Examples:
  Update the Ubuntu image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Ubuntu image location in ~/.lima/ubuntu/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") ~/.lima/ubuntu/lima.yaml
  $ limactl factory-reset ubuntu

  Update the Ubuntu image location to ubuntu-24.04-minimal-cloudimg-<arch>.img in ~/.lima/docker/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --minimal --version 24.04 ~/.lima/docker/lima.yaml
  $ limactl factory-reset docker

Flags:
  --flavor <flavor>    Use the specified flavor image
  --server             Shortcut for --flavor server
  --minimal            Shortcut for --flavor minimal
  --version <version>  Use the specified version
                       The version can be an alias: latest, latest_lts, or lts.
  -h, --help           Print this help message
HELP
}

readonly -A ubuntu_base_urls=(
	[minimal]=https://cloud-images.ubuntu.com/minimal/releases/
	[server]=https://cloud-images.ubuntu.com/releases/
)

# ubuntu_base_url prints the base URL for the given flavor.
# e.g.
# ```console
# ubuntu_base_url minimal
# https://cloud-images.ubuntu.com/minimal/releases/
# ```
function ubuntu_base_url() {
	[[ -v ubuntu_base_urls[$1] ]] || error_exit "Unsupported flavor: $1"
	echo "${ubuntu_base_urls[$1]}"
}

# ubuntu_downloaded_json downloads the JSON file for the given flavor and prints the path.
# e.g.
# ```console
# ubuntu_downloaded_json server
# /Users/user/Library/Caches/lima/download/by-url-sha256/255f982f5bbda07f5377369093e21c506d7240f5ba901479bdadfa205ddafb01/data
# ```
function ubuntu_downloaded_json() {
	local flavor=$1 base_url json_url
	json_url=$(ubuntu_base_url "${flavor}")streams/v1/com.ubuntu.cloud:released:download.json
	download_to_cache "${json_url}"
}
# ubuntu_image_url_try_replace_release_with_version tries to replace the release with the version in the URL.
# If the URL is valid, it prints the URL with the version.
function ubuntu_image_url_try_replace_release_with_version() {
	local location=$1 release=$2 version=$3 location_using_version
	set +e # Disable 'set -e' to avoid exiting on error for the next assignment.
	location_using_version=$(
		set -e
		validate_url "${location/\/${release}\//\/${version}\/}" 2>/dev/null
	) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
	# shellcheck disable=2181
	if [[ $? -eq 0 ]]; then
		echo "${location_using_version}"
	else
		echo "${location}"
	fi
	set -e
}

# ubuntu_image_url_latest prints the latest image URL and its digest for the given flavor, version, arch, and path suffix.
function ubuntu_image_url_latest() {
	local flavor=$1 version=$2 arch=$3 path_suffix=$4 base_url ubuntu_downloaded_json jq_filter location_digest_release
	base_url=$(ubuntu_base_url "${flavor}")
	ubuntu_downloaded_json=$(ubuntu_downloaded_json "${flavor}")
	jq_filter="
        [
            .products[\"com.ubuntu.cloud:${flavor}:${version}:${arch}\"] |
            .release as \$release |
            .versions[]?.items[] | select(.path | endswith(\"${path_suffix}\")) |
            [\"${base_url}\"+.path, \"sha256:\"+.sha256, \$release] | @tsv
        ] | last
    "
	location_digest_release=$(jq -r "${jq_filter}" "${ubuntu_downloaded_json}")
	[[ ${location_digest_release} != "null" ]] ||
		error_exit "The URL for ubuntu-${version}-${flavor}-cloudimg-${arch}${path_suffix} is not provided at ${ubuntu_base_urls[${flavor}]}."
	local location digest release location_using_version
	read -r location digest release <<<"${location_digest_release}"
	location=$(validate_url "${location}")
	location=$(ubuntu_image_url_try_replace_release_with_version "${location}" "${release}" "${version}")
	arch=$(limayaml_arch "${arch}")
	json_vars location arch digest
}

# ubuntu_image_url_release prints the release image URL for the given flavor, version, arch, and path suffix.
function ubuntu_image_url_release() {
	local flavor=$1 version=$2 arch=$3 path_suffix=$4 base_url
	base_url=$(ubuntu_base_url "${flavor}")
	ubuntu_downloaded_json=$(ubuntu_downloaded_json "${flavor}")
	local jq_filter release location
	jq_filter="
		[
            .products | to_entries[] as \$product_entry |
            \$product_entry.value| select(.version == \"${version}\") | 
            .release
		] | first
    "
	release=$(jq -r "${jq_filter}" "${ubuntu_downloaded_json}")
	[[ ${release} != "null" ]] ||
		error_exit "The URL for ubuntu-${version}-${flavor}-cloudimg-${arch}${path_suffix} is not provided at ${ubuntu_base_urls[${flavor}]}."
	location=$(validate_url "${base_url}${release}/release/ubuntu-${version}-${flavor}-cloudimg-${arch}${path_suffix}")
	location=$(ubuntu_image_url_try_replace_release_with_version "${location}" "${release}" "${version}")
	arch=$(limayaml_arch "${arch}")
	json_vars location arch
}

function ubuntu_file_info() {
	local location=$1 location_dirname sha256sums location_basename digest
	location=$(validate_url "${location}")
	location_dirname=$(dirname "${location}")
	sha256sums=$(download_to_cache "${location_dirname}/SHA256SUMS")
	location_basename=$(basename "${location}")
	# shellcheck disable=SC2034
	digest=${location+$(awk "/${location_basename}/{print \"sha256:\"\$1}" "${sha256sums}")}
	json_vars location digest
}

# ubuntu_image_entry_with_kernel_info prints image entry with kernel and initrd info.
# $1: image_entry
# e.g.
# ```console
# ubuntu_image_entry_with_kernel_info '{"location":"https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img"}'
# {"location":"https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img","kernel":{"location":"https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-vmlinuz-generic","digest":"sha256:..."}}
# ```
# shellcheck disable=SC2034
function ubuntu_image_entry_with_kernel_info() {
	local image_entry=$1 location
	location=$(jq -e -r '.location' <<<"${image_entry}")
	local location_dirname location_basename location_prefix
	location_dirname=$(dirname "${location}")/unpacked
	location_basename="$(basename "${location}" | cut -d- -f1-5 | cut -d. -f1-2)"
	location_prefix="${location_dirname}/${location_basename}"
	local kernel initrd
	set +e # Disable 'set -e' to avoid exiting on error for the next assignment.
	kernel=$(
		set -e
		ubuntu_file_info "${location_prefix}-vmlinuz-generic"
	) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
	# shellcheck disable=2181
	[[ $? -eq 0 ]] || error_exit "kernel image not found at ${location_prefix}-vmlinuz-generic"
	initrd=$(
		set -e
		ubuntu_file_info "${location_prefix}-initrd-generic" 2>/dev/null
	) # may not exist
	set -e
	json_vars kernel initrd <<<"${image_entry}"
}

function ubuntu_flavor_from_location_basename() {
	local location=$1 location_basename flavor
	location_basename=$(basename "${location}")
	flavor=$(echo "${location_basename}" | cut -d- -f3)
	[[ -n ${flavor} ]] || error_exit "Failed to get flavor from ${location}"
	echo "${flavor}"
}

function ubuntu_version_from_location_basename() {
	local location=$1 location_basename version
	location_basename=$(basename "${location}")
	version=$(echo "${location_basename}" | cut -d- -f2)
	[[ -n ${version} ]] || error_exit "Failed to get version from ${location}"
	echo "${version}"
}

# ubuntu_version_latest_lts prints the latest LTS version for the given flavor.
# e.g.
# ```console
# ubuntu_version_latest_lts minimal
# 24.04
# ```
function ubuntu_version_latest_lts() {
	local flavor=${1:-server}
	ubuntu_downloaded_json=$(ubuntu_downloaded_json "${flavor}")
	jq -e -r '[.products[]|.version|select(endswith(".04"))]|last // empty' "${ubuntu_downloaded_json}"
}

# ubuntu_version_latest prints the latest version for the given flavor.
# e.g.
# ```console
# ubuntu_version_latest minimal
# 24.10
# ```
function ubuntu_version_latest() {
	local flavor=${1:-server}
	ubuntu_downloaded_json=$(ubuntu_downloaded_json "${flavor}")
	jq -e -r '[.products[]|.version]|last // empty' "${ubuntu_downloaded_json}"
}

# ubuntu_version_resolve_aliases resolves the version aliases.
# e.g.
# ```console
# ubuntu_version_resolve_aliases https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img minimal latest
# 24.10
# ubuntu_version_resolve_aliases https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img minimal latest_lts
# 24.04
# ubuntu_version_resolve_aliases https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img
#
# ```
function ubuntu_version_resolve_aliases() {
	local location=$1 flavor version
	flavor=${2:-$(ubuntu_flavor_from_location_basename "${location}")}
	version=${3:-}
	case "${version}" in
	latest_lts | lts) ubuntu_version_latest_lts "${flavor}" ;;
	latest) ubuntu_version_latest "${flavor}" ;;
	*) echo "${version}" ;;
	esac
}

function ubuntu_arch_from_location_basename() {
	local location=$1 location_basename arch
	location_basename=$(basename "${location}")
	arch=$(echo "${location_basename}" | cut -d- -f5 | cut -d. -f1)
	[[ -n ${arch} ]] || error_exit "Failed to get arch from ${location}"
	echo "${arch}"
}

function ubuntu_path_suffix_from_location_basename() {
	local location=$1 arch path_suffix
	arch=$(ubuntu_arch_from_location_basename "${location}")
	path_suffix="${location##*"${arch}"}"
	[[ -n ${path_suffix} ]] || error_exit "Failed to get path suffix from ${location}"
	echo "${path_suffix}"
}

# ubuntu_location_url_spec prints the URL spec for the given location.
# If the location is not supported, it returns 1.
# e.g.
# ```console
# ubuntu_location_url_spec https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img
# latest
# ubuntu_location_url_spec https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
# release
# ```
function ubuntu_location_url_spec() {
	local location=$1 url_spec
	case "${location}" in
	https://cloud-images.ubuntu.com/minimal/releases/*/release/*) url_spec=release ;;
	https://cloud-images.ubuntu.com/minimal/releases/*/release-*/*) url_spec=latest ;;
	https://cloud-images.ubuntu.com/releases/*/release/*) url_spec=release ;;
	https://cloud-images.ubuntu.com/releases/*/release-*/*) url_spec=latest ;;
	*)
		# echo "Unsupported image location: ${location}" >&2
		return 1
		;;
	esac
	echo "${url_spec}"
}

# ubuntu_cache_key_for_image_kernel_flavor_version prints the cache key for the given location, kernel_location, flavor, and version.
# If the image location is not supported, it returns 1.
# kernel_location is not validated.
# e.g.
# ```console
# ubuntu_cache_key_for_image_kernel_flavor_version https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img
# ubuntu_latest_24.04-minimal-amd64-release-.img
# ubuntu_cache_key_for_image_kernel_flavor_version https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img https://...
# ubuntu_latest_with_kernel_24.04-minimal-amd64-release-.img
# ubuntu_cache_key_for_image_kernel_flavor_version https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img null
# ubuntu_release_24.04-server-amd64-.img
# ```
function ubuntu_cache_key_for_image_kernel_flavor_version() {
	local location=$1 kernel_location=${2:-null} url_spec with_kernel='' flavor version arch path_suffix
	url_spec=$(ubuntu_location_url_spec "${location}")
	[[ ${kernel_location} != "null" ]] && with_kernel=_with_kernel
	flavor=$(ubuntu_flavor_from_location_basename "${location}")
	version=$(ubuntu_version_from_location_basename "${location}")
	arch=$(ubuntu_arch_from_location_basename "${location}")
	path_suffix=$(ubuntu_path_suffix_from_location_basename "${location}")
	echo "ubuntu_${url_spec}${with_kernel}_${version}-${flavor}-${arch}-${path_suffix}"
}

function ubuntu_image_entry_for_image_kernel_flavor_version() {
	local location=$1 kernel_location=$2 url_spec
	url_spec=$(ubuntu_location_url_spec "${location}")

	local flavor version arch path_suffix
	flavor=${3:-$(ubuntu_flavor_from_location_basename "${location}")}
	version=${4:-$(ubuntu_version_from_location_basename "${location}")}
	arch=$(ubuntu_arch_from_location_basename "${location}")
	path_suffix=$(ubuntu_path_suffix_from_location_basename "${location}")

	local image_entry
	image_entry=$(ubuntu_image_url_"${url_spec}" "${flavor}" "${version}" "${arch}" "${path_suffix}")
	if [[ -z ${image_entry} ]]; then
		error_exit "Failed to get the ${url_spec} image location for ${location}"
	elif jq -e ".location == \"${location}\"" <<<"${image_entry}" >/dev/null; then
		echo "Image location is up-to-date: ${location}" >&2
	elif [[ ${kernel_location} != "null" ]]; then
		ubuntu_image_entry_with_kernel_info "${image_entry}"
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
		SUPPORTED_DISTRIBUTIONS+=("ubuntu")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("ubuntu")
	fi
	# required functions for Ubuntu
	function ubuntu_cache_key_for_image_kernel() { ubuntu_cache_key_for_image_kernel_flavor_version "$@"; }
	function ubuntu_image_entry_for_image_kernel() { ubuntu_image_entry_for_image_kernel_flavor_version "$@"; }

	return 0
fi

declare -a templates=()
declare overriding_flavor=
declare overriding_version=
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		ubuntu_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	--flavor)
		if [[ -n $2 && $2 != -* ]]; then
			overriding_flavor="$2"
			shift
		else
			error_exit "--flavor requires a value"
		fi
		;;
	--flavor=*) overriding_flavor="${1#*=}" ;;
	--minimal) overriding_flavor="minimal" ;;
	--server) overriding_flavor="server" ;;
	--version)
		if [[ -n $2 && $2 != -* ]]; then
			overriding_version="$2"
			shift
		else
			error_exit "--version requires a value"
		fi
		;;
	--version=*) overriding_version="${1#*=}" ;;
	*.yaml) templates+=("$1") ;;
	*)
		error_exit "Unknown argument: $1"
		;;
	esac
	shift
done

if [[ ${#templates[@]} -eq 0 ]]; then
	ubuntu_print_help
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
		overriding_version=$(
			set -e # Enable 'set -e' for the next command.
			ubuntu_version_resolve_aliases "${location}" "${overriding_flavor}" "${overriding_version}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		cache_key=$(
			set -e # Enable 'set -e' for the next command.
			ubuntu_cache_key_for_image_kernel_flavor_version "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				ubuntu_image_entry_for_image_kernel_flavor_version "${location}" "${kernel_location}" "${overriding_flavor}" "${overriding_version}"
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
