#!/usr/bin/env bash

set -eu -o pipefail

# Functions in this script assume error handling with 'set -e'.
# To ensure 'set -e' works correctly:
# - Use 'set +e' before assignments and '$(set -e; <function>)' to capture output without exiting on errors.
# - Avoid calling functions directly in conditions to prevent disabling 'set -e'.
# - Use 'shopt -s inherit_errexit' (Bash 4.4+) to avoid repeated 'set -e' in all '$(...)'.
shopt -s inherit_errexit || error_exit "inherit_errexit not supported. Please use bash 4.4 or later."

function archlinux_print_help() {
	cat <<HELP
$(basename "${BASH_SOURCE[0]}"): Update the Arch-Linux image location in the specified templates

Usage:
  $(basename "${BASH_SOURCE[0]}") <template.yaml>...

Description:
  This script updates the Arch-Linux image location in the specified templates.
  If the image location in the template contains a release date in the URL, the script replaces it with the latest available date.

  Image location basename format: Arch-Linux-<arch>-cloudimg[-<date>.<CI_JOB_ID>].qcow2

  Published Arch-Linux image information is fetched from the following URLs:

    x86_64:
      listing: https://gitlab.archlinux.org/api/v4/projects/archlinux%2Farch-boxes/packages
      details: https://gitlab.archlinux.org/api/v4/projects/archlinux%2Farch-boxes/packages/:package_id/package_files
    
    aarch64:
      https://github.com/mcginty/arch-boxes-arm/releases/
      
  Using 'gh' CLI tool for fetching the latest release from GitHub.

Examples:
  Update the Arch-Linux image location in templates/**.yaml:
  $ $(basename "${BASH_SOURCE[0]}") templates/**.yaml

  Update the Arch-Linux image location in ~/.lima/archlinux/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") ~/.lima/archlinux/lima.yaml
  $ limactl factory-reset archlinux

Flags:
  -h, --help              Print this help message
HELP
}

# print the URL spec for the given location
# shellcheck disable=SC2034
function archlinux_url_spec_from_location() {
	local location=$1 location_basename arch flavor source date_and_ci_job_id='' file_extension
	location_basename=$(basename "${location}")
	arch=$(echo "${location_basename}" | cut -d- -f3)
	flavor=$(echo "${location_basename}" | cut -d- -f4 | cut -d. -f1)
	case "${location}" in
	https://geo.mirror.pkgbuild.com/images/*)
		source="geo.mirror.pkgbuild.com"
		local -r date_and_ci_job_id_pattern='[0-9]{8}\.[0-9]+'
		if [[ ${location} =~ ${date_and_ci_job_id_pattern} ]]; then
			date_and_ci_job_id="${BASH_REMATCH[0]}"
		fi
		if [[ ${location_basename} =~ ${date_and_ci_job_id_pattern} ]]; then
			file_extension=${location_basename##*"${BASH_REMATCH[0]}".}
		else
			file_extension=${location_basename#*.}
		fi
		;;
	https://github.com/mcginty/arch-boxes-arm/releases/download/*)
		source="github.com/mcginty/arch-boxes-arm"
		local -r date_pattern='[0-9]{8}'
		if [[ ${location} =~ ${date_pattern} ]]; then
			date_and_ci_job_id="${BASH_REMATCH[0]}"
			file_extension=${location_basename#*"${date_and_ci_job_id}".*.}
		else
			error_exit "Failed to extract date from ${location}"
		fi
		;;
	*)
		# Unsupported location
		return 1
		;;
	esac
	json_vars source arch flavor date_and_ci_job_id file_extension
}

# print the location for the given URL spec
function archlinux_location_from_url_spec() {
	local url_spec=$1 source arch flavor date_and_ci_job_id file_extension location=''
	source=$(jq -r '.source' <<<"${url_spec}")
	arch=$(jq -r '.arch' <<<"${url_spec}")
	flavor=$(jq -r '.flavor' <<<"${url_spec}")
	date_and_ci_job_id=$(jq -r '.date_and_ci_job_id' <<<"${url_spec}")

	file_extension=$(jq -r '.file_extension' <<<"${url_spec}")
	case "${source}" in
	geo.mirror.pkgbuild.com)
		location="https://geo.mirror.pkgbuild.com/images/"
		if [[ -n ${date_and_ci_job_id} ]]; then
			location+="v${date_and_ci_job_id}/Arch-Linux-${arch}-${flavor}-${date_and_ci_job_id}.${file_extension}"
		else
			location+="latest/Arch-Linux-${arch}-${flavor}.${file_extension}"
		fi
		;;
	github.com/mcginty/arch-boxes-arm) ;;
	*)
		error_exit "Unsupported source: ${source}"
		;;
	esac
	echo "${location}"
}

# returns the image entry for the latest image in the gitlab mirror
function archlinux_image_entry_for_image_kernel_gitlab_mirror() {
	local location=$1 url_spec=$2 arch flavor date_and_ci_job_id gitlab_package_api_base latest_package_id jq_filter latest_package_file file_name digest updated_url_spec
	arch=$(jq -r '.arch' <<<"${url_spec}")
	if ! jq -e '.date_and_ci_job_id' <<<"${url_spec}" >/dev/null; then
		json_vars location arch
		return 1
	fi
	flavor=$(jq -r '.flavor' <<<"${url_spec}")
	date_and_ci_job_id=$(jq -r '.date_and_ci_job_id' <<<"${url_spec}")
	file_extension=$(jq -r '.file_extension' <<<"${url_spec}")
	gitlab_package_api_base="https://gitlab.archlinux.org/api/v4/projects/archlinux%2Farch-boxes/packages"
	latest_package_id=$(curl --silent --show-error "${gitlab_package_api_base}" | jq -r 'last|.id') || error_exit "Failed to fetch latest package_id"
	jq_filter="
        .[]|select(.file_name|test(\"^Arch-Linux-${arch}-${flavor}-.*\\\.${file_extension}\$\"))
    "
	latest_package_file=$(curl -s "${gitlab_package_api_base}/${latest_package_id}/package_files" | jq -r "${jq_filter}") ||
		error_exit "Failed to fetch latest package_file"
	file_name=$(jq -r '.file_name' <<<"${latest_package_file}")
	digest="sha256:$(jq -r '.file_sha256' <<<"${latest_package_file}")"
	local -r date_and_ci_job_id_pattern='[0-9]{8}\.[0-9]+'
	[[ ${file_name} =~ ${date_and_ci_job_id_pattern} ]] || error_exit "Failed to extract date_and_ci_job_id from ${file_name}"
	date_and_ci_job_id="${BASH_REMATCH[0]}"
	updated_url_spec=$(json_vars date_and_ci_job_id <<<"${url_spec}")
	location=$(archlinux_location_from_url_spec "${updated_url_spec}")
	location=$(validate_url_without_redirect "${location}")
	json_vars location arch digest
}

# returns the image entry for the latest image in the GitHub repo
function archlinux_image_entry_for_image_kernel_github_com() {
	local location=$1 url_spec=$2 arch flavor file_extension repo jq_filter latest_location downloaded_img digest
	arch=$(jq -r '.arch' <<<"${url_spec}")
	if ! jq -e '.date_and_ci_job_id' <<<"${url_spec}" >/dev/null; then
		json_vars location arch
		return 1
	fi
	flavor=$(jq -r '.flavor' <<<"${url_spec}")
	file_extension=$(jq -r '.file_extension' <<<"${url_spec}")
	command -v gh >/dev/null || error_exit "gh is required for fetching the latest release from GitHub, but it's not installed"
	local -r repo_pattern='github.com/(.*)/releases/download/(.*)'
	if [[ ${location} =~ ${repo_pattern} ]]; then
		repo="${BASH_REMATCH[1]}"
	else
		error_exit "Failed to extract repo and release from ${location}"
	fi
	jq_filter=".assets[]|select(.name|test(\"^Arch-Linux-${arch}-${flavor}-.*\\\.${file_extension}\$\"))|.url"
	latest_location=$(gh release view --repo "${repo}" --json assets --jq "${jq_filter}")
	[[ -n ${latest_location} ]] || error_exit "Failed to fetch the latest release URL from ${repo}"
	if [[ ${location} == "${latest_location}" ]]; then
		json_vars location arch
		return
	fi
	location=${latest_location}
	downloaded_img=$(download_to_cache_without_redirect "${latest_location}")
	# shellcheck disable=SC2034
	digest="sha512:$(sha512sum "${downloaded_img}" | cut -d' ' -f1)"
	json_vars location arch digest
}

function archlinux_cache_key_for_image_kernel() {
	local location=$1 url_spec
	url_spec=$(archlinux_url_spec_from_location "${location}")
	jq -r '["archlinux", .source, .arch, .date_and_ci_job_id // empty]|join(":")' <<<"${url_spec}"
}

function archlinux_image_entry_for_image_kernel() {
	local location=$1 url_spec source image_entry=''
	url_spec=$(archlinux_url_spec_from_location "${location}")
	source=$(jq -r '.source' <<<"${url_spec}")
	case "${source}" in
	geo.mirror.pkgbuild.com)
		image_entry=$(archlinux_image_entry_for_image_kernel_gitlab_mirror "${location}" "${url_spec}")
		;;
	github.com/mcginty/arch-boxes-arm)
		image_entry=$(archlinux_image_entry_for_image_kernel_github_com "${location}" "${url_spec}")
		;;
	*) error_exit "Unsupported source: ${source}" ;;
	esac
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
		SUPPORTED_DISTRIBUTIONS+=("archlinux")
	else
		declare -a SUPPORTED_DISTRIBUTIONS=("archlinux")
	fi
	return 0
fi

declare -a templates=()
declare overriding="{}"
while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		archlinux_print_help
		exit 0
		;;
	-d | --debug) set -x ;;
	*.yaml) templates+=("$1") ;;
	*)
		error_exit "Unknown argument: $1"
		;;
	esac
	shift
	[[ -z ${overriding} ]] && overriding="{}"
done

if [[ ${#templates[@]} -eq 0 ]]; then
	archlinux_print_help
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
			archlinux_cache_key_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
		) # Check exit status separately to prevent disabling 'set -e' by using the function call in the condition.
		# shellcheck disable=2181
		[[ $? -eq 0 ]] || continue
		image_entry=$(
			set -e # Enable 'set -e' for the next command.
			if [[ -v image_entry_cache[${cache_key}] ]]; then
				echo "${image_entry_cache[${cache_key}]}"
			else
				archlinux_image_entry_for_image_kernel "${location}" "${kernel_location}" "${overriding}"
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
