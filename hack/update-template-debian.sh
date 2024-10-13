#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function debian_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Debian image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--backports[=<bool>]] [--daily[=<bool>]] [--timestamped[=<bool>]] [--version <version>] <template.yaml>...

Description:
  This script updates the Debian image location in the specified templates.
  If the image location in the template contains a release date in the URL, the script replaces it with the latest available date.
  If no flags are specified, the script uses the version from the image location basename in the template.

  Image location basename format: debian-<version>[-backports]-genericcloud-<arch>[-daily][-<timestamp>].qcow2

  Published Debian image information is fetched from the following URLs:

    https://cloud.debian.org/images/cloud/<codename>[-backports]/[daily/](latest|<timestamp>)/debian-<version>[-backports]-genericcloud-<arch>[-daily][-<timestamp>].json

  The downloaded JSON file will be cached in the Lima cache directory.

Examples:
  Update the Debian image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Debian image location in ~/.lima/debian/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") ~/.lima/debian/lima.yaml
  $ limactl factory-reset debian

  Update the Debian image location to debian-13-genericcloud-<arch>.qcow2 in ~/.lima/debian/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version trixie ~/.lima/debian/lima.yaml
  $ limactl factory-reset debian

Flags:
  --backports[=<bool>]    Use the backports image
                          The boolean value can be true, false, 1, or 0
  --daily[=<bool>]        Use the daily image
  --timestamped[=<bool>]  Use the timestamped image
  --version <version>     Use the specified version
                          The version can be a codename, version number, or alias (testing, stable, oldstable)
  -h, --help              Print this help message
HELP
}

readonly debian_base_url=https://cloud.debian.org/images/cloud/

readonly debian_target_vendor=genericcloud

readonly -A debian_version_to_codename=(
	[10]=buster
	[11]=bullseye
	[12]=bookworm
	[13]=trixie
	[14]=forky
)

declare -A debian_codename_to_version
function debian_setup_codename_to_version() {
	local version codename
	for version in "${!debian_version_to_codename[@]}"; do
		codename=${debian_version_to_codename[${version}]}
		debian_codename_to_version[${codename}]="${version}"
	done
	readonly -A debian_codename_to_version
}
debian_setup_codename_to_version

readonly -A debian_alias_to_codename=(
	[testing]=trixie
	[stable]=bookworm
	[oldstable]=bullseye
)

# debian_downloaded_json downloads the JSON file for the given url_spec(JSON) and caches it
# e.g.
# ```console
# debian_downloaded_json '{"backports":false,"daily":false,"version":12,"arch":"amd64","file_extension":"qcow2"}'
#
# ```
function debian_downloaded_json() {
	local url_spec=$1 json_url_spec json_url
	json_url_spec=$(jq -r '. | del(.timestamp) | .file_extension = "json"' <<<"${url_spec}") || error_exit "Failed to create JSON URL spec"
	json_url=$(debian_location_from_url_spec "${json_url_spec}")
	download_to_cache "${json_url}"
}

function debian_digest_from_upload_entry() {
	local upload_entry=$1 debian_digest digest
	debian_digest=$(jq -e -r '.metadata.annotations."cloud.debian.org/digest"' <<<"${upload_entry}") ||
		error_exit "Failed to get the digest from ${upload_entry}"
	case "${debian_digest%:*}" in
	sha512) digest=$(echo "${debian_digest#*:}==" | base64 -d | xxd -p -c -) ||
		error_exit "Failed to decode the digest from ${debian_digest}" ;;
	*) error_exit "Unsupported digest type: ${debian_digest%:*}" ;;
	esac
	echo "${debian_digest/:*/:}${digest}"
}

# debian_image_url_timestamped prints the latest image URL and its digest for the given flavor, version, arch, and path suffix.
function debian_image_url_timestamped() {
	local url_spec=$1 debian_downloaded_json jq_filter upload_entry timestamp timestamped_url_spec location arch digest
	debian_downloaded_json=$(debian_downloaded_json "${url_spec}")
	# shellcheck disable=SC2016
	jq_filter='
		[.items[]|select(.kind == "Upload")|
		select(.metadata.labels."upload.cloud.debian.org/image-format" == $ARGS.named.url_spec.image_format)]|first
	'
	upload_entry=$(jq -e -r --argjson url_spec "${url_spec}" "${jq_filter}" "${debian_downloaded_json}") ||
		error_exit "Failed to find the upload entry from ${debian_downloaded_json}"
	timestamp=$(jq -e -r '.metadata.labels."cloud.debian.org/version"' <<<"${upload_entry}") ||
		error_exit "Failed to get the timestamp from ${upload_entry}"
	timestamped_url_spec=$(json_vars timestamp <<<"${url_spec}")
	location=$(debian_location_from_url_spec "${timestamped_url_spec}")
	location=$(validate_url_without_redirect "${location}")
	arch=$(jq -e -r '.arch' <<<"${url_spec}") || error_exit "missing arch in ${url_spec}"
	arch=$(limayaml_arch "${arch}")
	digest=$(debian_digest_from_upload_entry "${upload_entry}")
	json_vars location arch digest
}

# debian_image_url_not_timestamped prints the release image URL for the given url_spec(JSON)
function debian_image_url_not_timestamped() {
	local url_spec=$1 location arch
	location=$(debian_location_from_url_spec "${url_spec}")
	location=$(validate_url_without_redirect "${location}")
	arch=$(jq -e -r '.arch' <<<"${url_spec}") || error_exit "missing arch in ${url_spec}"
	arch=$(limayaml_arch "${arch}")
	json_vars location arch
}

# debian_version_resolve_aliases resolves the version aliases.
# e.g.
# ```console
# debian_version_resolve_aliases testing
# 13
# debian_version_resolve_aliases stable
# 12
# debian_version_resolve_aliases oldstable
# 11
# debian_version_resolve_aliases bookworm
# 12
# debian_version_resolve_aliases 10
# 10
# debian_version_resolve_aliases ''
#
# ```
function debian_version_resolve_aliases() {
	local version=$1
	[[ -v debian_alias_to_codename[${version}] ]] && version=${debian_alias_to_codename[${version}]}
	[[ -v debian_codename_to_version[${version}] ]] && version=${debian_codename_to_version[${version}]}
	[[ -v debian_version_to_codename[${version}] ]] || error_exit "Unsupported version: ${version}"
	[[ -z ${version} ]] || echo "${version}"
}

function debian_arch_from_location_basename() {
	local location=$1 location_basename arch
	location_basename=$(basename "${location}")
	location_basename=${location_basename/-backports/}
	arch=$(echo "${location_basename}" | cut -d- -f4 | cut -d. -f1)
	[[ -n ${arch} ]] || error_exit "Failed to get arch from ${location}"
	echo "${arch}"
}

function debian_file_extension_from_location_basename() {
	local location=$1 location_basename file_extension
	location_basename=$(basename "${location}")
	file_extension=$(echo "${location_basename}" | cut -d. -f2-) # remove the first field
	[[ -n ${file_extension} ]] || error_exit "Failed to get file extension from ${location}"
	echo "${file_extension}"
}

function debian_image_format_from_file_extension() {
	local file_extension=$1
	case "${file_extension}" in
	json) echo "json" ;;
	qcow2) echo "qcow2" ;;
	raw) echo "raw" ;;
	tar.xz) echo "internal" ;;
	*) error_exit "Unsupported file extension: ${file_extension}" ;;
	esac
}

# debian_url_spec_from_location returns the URL spec for the given location.
# If the location is not supported, it returns 1.
# e.g.
# ```console
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2
# {"backports":false,"daily":false,"version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm/20241004-1890/debian-12-generic-amd64-20241004-1890.qcow2
# {"backports":false,"daily":false,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm/daily/latest/debian-12-genericcloud-amd64-daily.qcow2
# {"backports":false,"daily":true,"version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm/daily/20241019-1905/debian-12-genericcloud-amd64-daily-20241019-1905.qcow2
# {"backports":false,"daily":true,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm-backports/latest/debian-12-backports-genericcloud-amd64.qcow2
# {"backports":true,"daily":false,"version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm-backports/20241004-1890/debian-12-backports-genericcloud-amd64-20241004-1890.qcow2
# {"backports":true,"daily":false,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm-backports/daily/latest/debian-12-backports-genericcloud-amd64-daily.qcow2
# {"backports":true,"daily":true,"version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# debian_url_spec_from_location https://cloud.debian.org/images/cloud/bookworm-backports/daily/20241019-1905/debian-12-backports-genericcloud-amd64-daily-20241019-1905.qcow2
# {"backports":true,"daily":true,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2","image_format":"qcow2"}
# ```
# shellcheck disable=SC2034
function debian_url_spec_from_location() {
	local location=$1 backports=false daily=false timestamp='' codename version='' arch file_extension image_format
	local -r timestamp_pattern='[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9]'
	case "${location}" in
	${debian_base_url}*-backports/*) backports=true ;;&
	${debian_base_url}*/daily/*) daily=true ;;&
	${debian_base_url}*/${timestamp_pattern}/*) [[ ${location} =~ ${timestamp_pattern} ]] && timestamp=${BASH_REMATCH[0]} ;;
	${debian_base_url}*/latest/*) timestamp='' ;;
	*)
		# echo "Unsupported image location: ${location}" >&2
		return 1
		;;
	esac
	codename=$(echo "${location#"${debian_base_url}"}" | cut -d/ -f1 | cut -d- -f1)
	[[ -v debian_codename_to_version[${codename}] ]] || error_exit "Unknown codename: ${codename}"
	version=${debian_codename_to_version[${codename}]}
	arch=$(debian_arch_from_location_basename "${location}")
	file_extension=$(debian_file_extension_from_location_basename "${location}")
	image_format=$(debian_image_format_from_file_extension "${file_extension}")
	json_vars backports daily timestamp version arch file_extension image_format
}

# debian_location_from_url_spec returns the location for the given URL spec.
# e.g.
# ```console
# debian_location_from_url_spec '{"backports":false,"daily":false,"version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2
# debian_location_from_url_spec '{"backports":false,"daily":false,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm/20241019-1905/debian-12-generic-amd64-20241019-1905.qcow2
# debian_location_from_url_spec '{"backports":false,"daily":true,"version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm/daily/latest/debian-12-genericcloud-amd64-daily.qcow2
# debian_location_from_url_spec '{"backports":false,"daily":true,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm/daily/20241019-1905/debian-12-generic-amd64-daily-20241019-1905.qcow2
# debian_location_from_url_spec '{"backports":true,"daily":false,"version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm-backports/latest/debian-12-backports-genericcloud-amd64.qcow2
# debian_location_from_url_spec '{"backports":true,"daily":false,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm-backports/20241019-1905/debian-12-backports-genericcloud-amd64-20241019-1905.qcow2
# debian_location_from_url_spec '{"backports":true,"daily":true,"version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm-backports/daily/latest/debian-12-backports-genericcloud-amd64-daily.qcow2
# debian_location_from_url_spec '{"backports":true,"daily":true,"timestamp":"20241019-1905","version":12,"arch":"amd64","file_extension":"qcow2"}'
# https://cloud.debian.org/images/cloud/bookworm-backports/daily/20241019-1905/debian-12-backports-genericcloud-amd64-daily-20241019-1905.qcow2
# ```
function debian_location_from_url_spec() {
	local url_spec=$1 base_url version backports daily timestamp arch file_extension
	base_url=${debian_base_url}
	version=$(jq -e -r '.version' <<<"${url_spec}")
	[[ -v debian_version_to_codename[${version}] ]] || error_exit "Unsupported version: ${version}"
	base_url+=${debian_version_to_codename[${version}]}
	backports=$(jq -r 'if .backports then "-backports" else empty end' <<<"${url_spec}")
	base_url+=${backports}/
	daily=$(jq -r 'if .daily then "daily" else empty end' <<<"${url_spec}")
	base_url+=${daily:+${daily}/}
	timestamp=$(jq -r 'if .timestamp then .timestamp else empty end' <<<"${url_spec}")
	base_url+=${timestamp:-latest}/
	arch=$(jq -e -r '.arch' <<<"${url_spec}")
	file_extension=$(jq -e -r '.file_extension' <<<"${url_spec}")
	base_url+=debian-${version}${backports}-${debian_target_vendor}-${arch}${daily:+-${daily}}${timestamp:+-${timestamp}}.${file_extension}
	echo "${base_url}"
}

# debian_cache_key_for_image_kernel_overriding returns the cache key for the given location, kernel_location, flavor, and version.
# If the image location is not supported, it returns 1.
# kernel_location is not validated.
# e.g.
# ```console
# debian_cache_key_for_image_kernel_overriding https://cloud-images.debian.com/minimal/releases/24.04/release-20210914/debian-24.04-minimal-cloudimg-amd64.img
# debian_latest_24.04-minimal-amd64-release-.img
# debian_cache_key_for_image_kernel_overriding https://cloud-images.debian.com/minimal/releases/24.04/release-20210914/debian-24.04-minimal-cloudimg-amd64.img https://...
# debian_latest_with_kernel_24.04-minimal-amd64-release-.img
# debian_cache_key_for_image_kernel_overriding https://cloud-images.debian.com/releases/24.04/release/debian-24.04-server-cloudimg-amd64.img null
# debian_release_24.04-server-amd64-.img
# ```
function debian_cache_key_for_image_kernel_overriding() {
	local location=$1 kernel_location=${2:-null} overriding=${3:-"{}"} url_spec with_kernel='' version backports arch daily timestamped file_extension
	url_spec=$(debian_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	[[ ${kernel_location} != "null" ]] && with_kernel=_with_kernel
	version=$(jq -r '.version|if . then "-\(.)" else empty end' <<<"${url_spec}")
	backports=$(jq -r 'if .backports then "-backports" else empty end' <<<"${url_spec}")
	arch=$(jq -e -r '.arch' <<<"${url_spec}")
	daily=$(jq -r 'if .daily then "-daily" else empty end' <<<"${url_spec}")
	timestamped=$(jq -r 'if .timestamp then "-timestamped" else empty end' <<<"${url_spec}")
	file_extension=$(jq -e -r '.file_extension' <<<"${url_spec}")
	echo "debian${with_kernel}${version}${backports}-${debian_target_vendor}-${arch}${daily}${timestamped}.${file_extension}"
}

function debian_image_entry_for_image_kernel_overriding() {
	local location=$1 kernel_location=$2 overriding=${3:-"{}"} url_spec timestamped
	[[ ${kernel_location} == "null" ]] || error_exit "Updating image with kernel is not supported"
	url_spec=$(debian_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	timestamped=$(jq -r 'if .timestamp then "timestamped" else "not_timestamped" end' <<<"${url_spec}")

	local image_entry
	image_entry=$(debian_image_url_"${timestamped}" "${url_spec}")
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
		SUPPORTED_DISTRIBUTIONS+=("debian")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("debian")
	fi
	# required functions for Debian
	function debian_cache_key_for_image_kernel() { debian_cache_key_for_image_kernel_overriding "$@"; }
	function debian_image_entry_for_image_kernel() { debian_image_entry_for_image_kernel_overriding "$@"; }

	return 0
fi

declare -a templates=()
declare overriding="{}"
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		debian_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	--backports | --daily | --timestamped)
		overriding=$(json "${1#--}" true <<<"${overriding}")
		;;
	--backports=* | --daily=* | --timestamped=*)
		overriding=$(
			key=${1#--} value=$(validate_boolean "${1#*=}")
			json "${key%%=*}" "${value}" <<<"${overriding}"
		)
		;;
	--version)
		if [[ -n $2 && $2 != -* ]]; then
			overriding=$(
				version=$(debian_version_resolve_aliases "$2")
				json_vars version <<<"${overriding}"
			)
			shift
		else
			error_exit "--version requires a value"
		fi
		;;
	--version=*)
		overriding=$(
			version=$(debian_version_resolve_aliases "${1#*=}")
			json_vars version <<<"${overriding}"
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
	debian_print_help
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
			debian_cache_key_for_image_kernel_overriding "${location}" "${kernel_location}" "${overriding}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				debian_image_entry_for_image_kernel_overriding "${location}" "${kernel_location}" "${overriding}"
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
