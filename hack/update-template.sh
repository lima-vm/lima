#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") <template.yaml>...

Description:
  This script updates the image location in the specified templates.
  If the image location in the template contains a release date in the URL, the script replaces it with the latest available date.

Examples:
  Update the Ubuntu image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Ubuntu image location in ~/.lima/ubuntu/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") ~/.lima/ubuntu/lima.yaml
  $ limactl factory-reset ubuntu

Flags:
  -h, --help           Print this help message
HELP
}

# json prints the JSON object with the given arguments.
# json [key value ...]
# if the value is empty, both key and value are omitted.
# e.g.
# ```console
# json location https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img arch amd64 digest sha256:...
# {"location":"https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img","arch":"amd64","digest":"sha256:..."}
# ```
function json() {
	local args=() pattern='^(\[.*\]|\{.*\}|true|false|[0-9]+)$' value
	[[ ! -p /dev/stdin ]] && args+=(--null-input)
	while [[ $# -gt 0 ]]; do
		value="${2-}"
		if [[ ${value} =~ ${pattern} ]]; then
			args+=(--argjson "${1}" "${value}")
		elif [[ -n ${value} ]]; then
			args+=(--arg "${1}" "${value}")
		fi # omit empty values
		shift
		shift # shift 2 does not work when $# is 1
	done
	jq -c "${args[@]}" '. + $ARGS.named | if . == {} then empty else . end'
}

# json_vars prints the JSON object with the given variable names.
# e.g.
# ```console
# location=https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img
# arch=amd64
# digest=sha256:...
# json_vars location arch digest
# {"location":"https://cloud-images.ubuntu.com/minimal/releases/24.04/release-20210914/ubuntu-24.04-minimal-cloudimg-amd64.img","arch":"amd64","digest":"sha256:..."}
# ```
function json_vars() {
	local args=() var
	for var in "$@"; do
		[[ -v ${var} ]] || error_exit "${var} is not set"
		args+=("${var}" "${!var}")
	done
	json "${args[@]}"
}

# limayaml_arch prints the arch in the lima.yaml format
function limayaml_arch() {
	local arch=$1
	arch=${arch/amd64/x86_64}
	arch=${arch/arm64/aarch64}
	arch=${arch/armhf/armv7l}
	echo "${arch}"
}

function validate_boolean() {
	local value=$1
	case "${value}" in
	'') ;;
	true | 1) echo true ;;
	false | 0) echo false ;;
	*) error_exit "Invalid boolean value: ${value}" ;;
	esac
}

# validate_url checks if the URL is valid and prints the location if it is.
# If the URL is redirected, it prints the redirected location.
# e.g.
# ```console
# validate_url https://cloud-images.ubuntu.com/server/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
# https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
# ```
function validate_url() {
	local url=$1
	code_location=$(curl -sSL -o /dev/null -I -w "%{http_code}\t%{url_effective}" "${url}")
	read -r code location <<<"${code_location}"
	[[ ${code} -eq 200 ]] || error_exit "[${code}]: The image is not available for download from ${location}"
	echo "${location}"
}

# validate_url_without_redirect checks if the URL is valid and prints the location if it is.
# If the URL is redirected, it prints the URL before the redirection.
# e.g.
# ```console
# validate_url_without_redirect https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-arm64.qcow2
# https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-arm64.qcow2
# ```
# cloud.debian.org may be redirected to other domains(e.g. chuangtzu.ftp.acc.umu.se), but we want to use the original URL.
function validate_url_without_redirect() {
	local url=$1 location
	location=$(validate_url "${url}")
	[[ -n ${location} ]] || error_exit "The image is not available for download from ${url}"
	echo "${url}"
}

# check if the script is executed or sourced
# shellcheck disable=SC1091
if [[ ${BASH_SOURCE[0]} == "${0}" ]]; then
	scriptdir=$(dirname "${BASH_SOURCE[0]}")
	# shellcheck source=./cache-common-inc.sh
	. "${scriptdir}/cache-common-inc.sh"

	# Scripts for each distribution are expected to:
	# - Add their identifier to the SUPPORTED_DISTRIBUTIONS array.
	# - Register the following functions:
	#   - ${distribution}_cache_key_for_image_kernel
	#     - Arguments: location, kernel_location
	#     - Returns: cache_key (string)
	#     - Exits with an error if the image location is not supported.
	#   - ${distribution}_image_entry_for_image_kernel
	#     - Arguments: location, kernel_location
	#     - Returns: image_entry (JSON object)
	#	  - Exits with an error if the image location is not supported.
	declare -a SUPPORTED_DISTRIBUTIONS=()

	# shellcheck source=./update-template-ubuntu.sh
	. "${scriptdir}/update-template-ubuntu.sh"
	# shellcheck source=./update-template-debian.sh
	. "${scriptdir}/update-template-debian.sh"
	# shellcheck source=./update-template-archlinux.sh
	. "${scriptdir}/update-template-archlinux.sh"
	# shellcheck source=./update-template-centos-stream.sh
	. "${scriptdir}/update-template-centos-stream.sh"
else
	# this script is sourced
	return 0
fi

declare -a templates=()
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		print_help
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
	print_help
	exit 0
fi

declare -a distributions=()
# Check if the distribution has the required functions
for distribution in "${SUPPORTED_DISTRIBUTIONS[@]}"; do
	if declare -f "${distribution}_cache_key_for_image_kernel" >/dev/null &&
		declare -f "${distribution}_image_entry_for_image_kernel" >/dev/null; then
		distributions+=("${distribution}")
	fi
done
[[ ${#distributions[@]} -gt 0 ]] || error_exit "No supported distributions found"

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
		for distribution in "${distributions[@]}"; do
			set +e # Disable 'set -e' to avoid exiting on error for the next assignment.
			cache_key=$(
				set -e # Enable 'set -e' for the next command.
				"${distribution}_cache_key_for_image_kernel" "${location}" "${kernel_location}"
			) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
			# shellcheck disable=2181
			[[ $? -eq 0 ]] || continue
			image_entry=$(
				set -e # Enable 'set -e' for the next command.
				if [[ -v image_entry_cache[${cache_key}] ]]; then
					echo "${image_entry_cache[${cache_key}]}"
				else
					"${distribution}_image_entry_for_image_kernel" "${location}" "${kernel_location}"
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
done
