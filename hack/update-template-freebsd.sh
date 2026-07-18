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

function freebsd_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the FreeBSD image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") [--version <major>.<minor>] <template.yaml>...

Description:
  This script updates the FreeBSD image location in the specified templates.
  Image location basename format:

    FreeBSD-<version>-RELEASE-<arch>[-BASIC-CLOUDINIT]-<fs>.<format>.xz

  Published FreeBSD image information is fetched from the following URL:

    ${freebsd_archive_url}

  The downloaded files will be cached in the Lima cache directory.

Examples:
  Update the FreeBSD image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the FreeBSD image location to version 14.3 in ~/.lima/freebsd/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --version 14.3 ~/.lima/freebsd/lima.yaml
  $ limactl factory-reset freebsd

Flags:
  --version <major>.<minor>  Use the specified <major>.<minor> version.
  -h, --help                 Print this help message
HELP
}

# ftp-archive.freebsd.org doesn't seem to support HTTPS
readonly freebsd_archive_url='http://ftp-archive.freebsd.org/pub/FreeBSD-Archive/old-releases/VM-IMAGES/'

# freebsd_url_spec_from_location prints the URL spec for the given location.
# If the location is not supported, it returns 1.
# e.g.
# ```console
# freebsd_url_spec_from_location http://ftp-archive.freebsd.org/pub/FreeBSD-Archive/old-releases/VM-IMAGES/15.0-RELEASE/amd64/Latest/FreeBSD-15.0-RELEASE-amd64-BASIC-CLOUDINIT-zfs.raw.xz
# {"version":"15.0","dir_arch":"amd64","filename_arch":"amd64","cloudinit":true,"fs":"zfs","format":"raw"}
# freebsd_url_spec_from_location http://ftp-archive.freebsd.org/pub/FreeBSD-Archive/old-releases/VM-IMAGES/15.0-RELEASE/aarch64/Latest/FreeBSD-15.0-RELEASE-arm64-aarch64-BASIC-CLOUDINIT-zfs.raw.xz
# {"version":"15.0","dir_arch":"aarch64","filename_arch":"arm64-aarch64","cloudinit":true,"fs":"zfs","format":"raw"}
# ```
function freebsd_url_spec_from_location() {
	local location=$1 jq_filter url_spec
	jq_filter='capture("
		^http://ftp-archive\\.freebsd\\.org/pub/FreeBSD-Archive/old-releases/VM-IMAGES/
		(?<version>\\d+\\.\\d+)-RELEASE/(?<dir_arch>[^/]+)/Latest/
		FreeBSD-\\d+\\.\\d+-RELEASE-(?<filename_arch>arm64-aarch64|riscv-riscv64|amd64)
		(?<cloudinit>-BASIC-CLOUDINIT)?-(?<fs>zfs|ufs)\\.(?<format>raw|qcow2)\\.xz$
	";"x") | .cloudinit = (.cloudinit != null)
	'
	url_spec=$(jq -e -r "${jq_filter}" <<<"\"${location}\"")
	echo "${url_spec}"
}

readonly freebsd_jq_filter_directory='"http://ftp-archive.freebsd.org/pub/FreeBSD-Archive/old-releases/VM-IMAGES/\(.version)-RELEASE/\(.dir_arch)/Latest/"'
readonly freebsd_jq_filter_filename='
	"FreeBSD-\(.version)-RELEASE-\(.filename_arch)\(if .cloudinit then "-BASIC-CLOUDINIT" else "" end)-\(.fs).\(.format).xz"
'

# freebsd_location_from_url_spec prints the location for the given URL spec.
function freebsd_location_from_url_spec() {
	local url_spec=$1
	jq -e -r "${freebsd_jq_filter_directory} + ${freebsd_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the location for ${url_spec}"
}

function freebsd_directory_from_url_spec() {
	local url_spec=$1
	jq -e -r "${freebsd_jq_filter_directory}" <<<"${url_spec}" ||
		error_exit "Failed to get the directory for ${url_spec}"
}

function freebsd_filename_from_url_spec() {
	local url_spec=$1
	jq -e -r "${freebsd_jq_filter_filename}" <<<"${url_spec}" ||
		error_exit "Failed to get the filename for ${url_spec}"
}

function freebsd_latest_image_entry_for_url_spec() {
	local url_spec=$1 releases_page latest_version newer_url_spec location filename checksum_url downloaded_checksum digest arch
	releases_page=$(download_to_cache "${freebsd_archive_url}")

	# Find the latest RELEASE version with the same major version as in url_spec
	latest_version=$(jq -e -Rrs --argjson spec "${url_spec}" '
		[
			split("\n").[] |
			select(test("href=\"\\d+\\.\\d+-RELEASE/\"")) |
			capture("href=\"(?<ver>\\d+\\.\\d+)-RELEASE/\"") |
			.ver |
			select((split(".")[0] | tonumber) == ($spec.version | split(".")[0] | tonumber))
		] |
		sort_by(split(".") | map(tonumber)) |
		last
	' <"${releases_page}") ||
		error_exit "No RELEASE found for FreeBSD $(jq -r '.version | split(".")[0]' <<<"${url_spec}").x in ${freebsd_archive_url}"
	[[ -n ${latest_version} ]] || return
	newer_url_spec=$(jq -e -r ". + {version: \"${latest_version}\"}" <<<"${url_spec}")
	filename=$(freebsd_filename_from_url_spec "${newer_url_spec}")
	checksum_url="$(freebsd_directory_from_url_spec "${newer_url_spec}")CHECKSUM.SHA256"
	downloaded_checksum=$(download_to_cache "${checksum_url}")
	digest="sha256:$(awk "/SHA256 \\(${filename}\\) =/{print \$4}" "${downloaded_checksum}")"
	[[ -n ${digest} ]] || error_exit "Failed to get the digest for ${filename}"
	location=$(freebsd_location_from_url_spec "${newer_url_spec}")
	location=$(validate_url "${location}")
	arch=$(jq -e -r '.dir_arch' <<<"${newer_url_spec}")
	arch=$(limayaml_arch "${arch}")
	json_vars location arch digest
}

function freebsd_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(freebsd_url_spec_from_location "${location}")
	jq -r '["freebsd", (.version | split(".")[0]), .dir_arch, (if .cloudinit then "cloudinit" else "" end), .fs, .format] | join(":")' <<<"${url_spec}"
}

function freebsd_image_entry_for_image_kernel() {
	local location=$1 kernel_is_not_supported=$2 overriding=${3:-'{}'} url_spec image_entry=''
	[[ ${kernel_is_not_supported} == "null" ]] || echo "Updating kernel information is not supported on FreeBSD" >&2
	url_spec=$(freebsd_url_spec_from_location "${location}" | jq -r ". + ${overriding}")
	image_entry=$(freebsd_latest_image_entry_for_url_spec "${url_spec}")
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
		SUPPORTED_DISTRIBUTIONS+=("freebsd")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("freebsd")
	fi
	return 0
fi

declare -a templates=()
declare overriding='{}'
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		freebsd_print_help
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
			[[ ${version} =~ ^[0-9]+\.[0-9]+$ ]] ||
				error_exit "The version must be in the format <major>.<minor>"
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
	freebsd_print_help
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
			freebsd_cache_key_for_image_kernel "${location}" "${kernel_location}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				freebsd_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
