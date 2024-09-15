#!/usr/bin/env bash

function print_help() {
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

  Update the Ubuntu image location to ubuntu-24.04-minimal-cloudimg-<arch>.img in ~/.lima/docker/lima.yaml:
  $ $(basename "${BASH_SOURCE[0]}") --minimal --version 24.04 ~/.lima/docker/lima.yaml

Flags:
  --flavor <flavor>    Use the specified flavor image
  --server             Shortcut for --flavor server
  --minimal            Shortcut for --flavor minimal
  --version <version>  Use the specified version
  -h, --help           Print this help message
HELP
}

scriptdir=$(dirname "${BASH_SOURCE[0]}")
# shellcheck source=./cache-common-inc.sh
# shellcheck disable=SC1091
. "${scriptdir}/cache-common-inc.sh"

set -eu -o pipefail

readonly -A base_urls=(
	[minimal]=https://cloud-images.ubuntu.com/minimal/releases/
	[server]=https://cloud-images.ubuntu.com/releases/
)

# validate_url checks if the URL is valid and returns the location if it is.
# If the URL is redirected, it returns the redirected location.
# e.g.
# ```console
# validate_url https://cloud-images.ubuntu.com/server/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
# https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
# ```
function validate_url() {
	local url=$1
	code_location=$(curl -sSL -o /dev/null -I -w "%{http_code}\t%{url_effective}" "${url}")
	read -r code location <<<"${code_location}"
	[[ ${code} -eq 200 ]] && echo "${location}"
}

# ubuntu_base_url returns the base URL for the given flavor.
# e.g.
# ```console
# ubuntu_base_url minimal
# https://cloud-images.ubuntu.com/minimal/releases/
# ```
function ubuntu_base_url() {
	# shellcheck disable=SC2015
	[[ -v base_urls[$1] ]] && echo "${base_urls[$1]}" || (
		echo "Unsupported flavor: $1" >&2
		exit 1
	)
}

# downloaded_json downloads the JSON file for the given flavor and returns the path.
# e.g.
# ```console
# downloaded_json server
# /Users/user/Library/Caches/lima/download/by-url-sha256/255f982f5bbda07f5377369093e21c506d7240f5ba901479bdadfa205ddafb01/data
# ```
function downloaded_json() {
	local flavor=$1 base_url json_url
	json_url=$(ubuntu_base_url "${flavor}")streams/v1/com.ubuntu.cloud:released:download.json
	download_to_cache "${json_url}"
}

# ubuntu_image_url_try_replace_release_with_version tries to replace the release with the version in the URL.
# If the URL is valid, it returns the URL with the version.
function ubuntu_image_url_try_replace_release_with_version() {
	local location=$1 release=$2 version=$3 location_using_version
	# shellcheck disable=SC2310
	if location_using_version=$(validate_url "${location/\/${release}\//\/${version}\/}"); then
		echo "${location_using_version}"
	else
		echo "${location}"
	fi
}

# ubuntu_image_url_latest returns the latest image URL and its digest for the given version, flavor, arch, and path suffix.
function ubuntu_image_url_latest() {
	local version=$1 flavor=$2 arch=$3 path_suffix=$4 base_url downloaded_json jq_filter location_digest_release
	base_url=$(ubuntu_base_url "${flavor}")
	# shellcheck disable=SC2310
	downloaded_json=$(downloaded_json "${flavor}") || return 0
	jq_filter="
        [
            .products[\"com.ubuntu.cloud:${flavor}:${version}:${arch}\"] |
            .release as \$release |
            .versions[]?.items[] | select(.path | endswith(\"${path_suffix}\")) |
            [\"${base_url}\"+.path, \"sha256:\"+.sha256, \$release] | @tsv
        ] | last
    "
	location_digest_release=$(jq -e -r "${jq_filter}" "${downloaded_json}") || return 0
	local location digest release location_using_version
	read -r location digest release <<<"${location_digest_release}"
	# shellcheck disable=SC2310
	location=$(validate_url "${location}") || return 0
	location=$(ubuntu_image_url_try_replace_release_with_version "${location}" "${release}" "${version}")
	echo -e "${location}\t${digest}"
}

# ubuntu_image_url_release returns the release image URL for the given version, flavor, arch, and path suffix.
function ubuntu_image_url_release() {
	local version=$1 flavor=$2 arch=$3 path_suffix=$4 base_url
	base_url=$(ubuntu_base_url "${flavor}")
	# shellcheck disable=SC2310
	downloaded_json=$(downloaded_json "${flavor}") || return 0
	local location release location_using_version
	jq_filter="
		[
            .products | to_entries[] as \$product_entry |
            \$product_entry.value| select(.version == \"${version}\") | 
            .release
		] | first
    "
	release=$(jq -e -r "${jq_filter}" "${downloaded_json}") || return 0
	# shellcheck disable=SC2310
	location=$(validate_url "${base_url}${release}/release/ubuntu-${version}-${flavor}-cloudimg-${arch}${path_suffix}") || return 0
	ubuntu_image_url_try_replace_release_with_version "${location}" "${release}" "${version}"
}

# ubuntu_kernel_info_for_image_url returns the kernel and initrd location and digest for the given location.
function ubuntu_kernel_info_for_image_url() {
	local location=$1 location_dirname sha256sums location_basename
	location_dirname=$(dirname "${location}")/unpacked
	sha256sums=$(curl -sSLf "${location_dirname}/SHA256SUMS")
	location_basename="$(basename "${location}" | cut -d- -f1-5 | cut -d. -f1-2)"

	# kernel
	local kernel_basename kernel_location kernel_digest
	kernel_basename="${location_basename}-vmlinuz-generic"
	# shellcheck disable=SC2310
	kernel_location=$(validate_url "${location_dirname}/${kernel_basename}") || return 0
	kernel_digest=${kernel_location+$(awk "/${kernel_basename}/{print \"sha256:\"\$1}" <<<"${sha256sums}")}

	# initrd
	local initrd_basename initrd_location initrd_digest
	initrd_basename="${location_basename}-initrd-generic"
	initrd_location=$(validate_url "${location_dirname}/${initrd_basename}")
	initrd_digest=${initrd_location+$(awk "/${initrd_basename}/{print \"sha256:\"\$1}" <<<"${sha256sums}")}

	echo -e "${kernel_location}\t${kernel_digest}\t${initrd_location}\t${initrd_digest}"
}

# limayaml_arch returns the arch in the lima.yaml format
function limayaml_arch() {
	local arch=$1
	arch=${arch/amd64/x86_64}
	arch=${arch/arm64/aarch64}
	arch=${arch/armhf/armv7l}
	echo "${arch}"
}

declare -a templates=()

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		print_help
		exit 0
		;;
	--flavor)
		if [[ -n $2 && $2 != -* ]]; then
			flavor="$2"
			shift
		else
			echo "Error: --flavor requires a value" >&2
			exit 1
		fi
		;;
	--flavor=*) flavor="${1#*=}" ;;
	--minimal) flavor="minimal" ;;
	--server) flavor="server" ;;
	--version)
		if [[ -n $2 && $2 != -* ]]; then
			version="$2"
			shift
		else
			echo "Error: --version requires a value" >&2
			exit 1
		fi
		;;
	--version=*) version="${1#*=}" ;;
	*.yaml) templates+=("$1") ;;
	*)
		echo "Unknown argument: $1" >&2
		exit 1
		;;
	esac
	shift
done

if [[ ${#templates[@]} -eq 0 ]]; then
	print_help
	exit 0
fi

flavor=${flavor:-server}
downloaded_json=$(downloaded_json "${flavor}")
version="${version:-$(jq -r '[.products[]|.version|select(endswith(".04"))]|last' "${downloaded_json}")}"

declare -A ubuntu_image_url_latest_cache=()
declare -A ubuntu_image_url_release_cache=()

for template in "${templates[@]}"; do
	echo "Processing ${template}"
	# 1. extract location by parsing template using arch
	yq_filter="
		.images[] | [.location, .kernel.location, .kernel.cmdline, .initrd.location] | @tsv
	"
	parsed=$(yq eval "${yq_filter}" "${template}")

	# 3. get the image location
	arr=()
	while IFS= read -r line; do arr+=("${line}"); done <<<"${parsed}"
	locations=("${arr[@]}")
	for ((index = 0; index < ${#locations[@]}; index++)); do
		[[ ${locations[index]} != "null" ]] || continue
		IFS=$'\t' read -r location kernel_location kernel_cmdline initrd_location <<<"${locations[index]}"
		location_before="${location}"

		case "${location}" in
		https://cloud-images.ubuntu.com/minimal/releases/*/release/*) use_latest=0 ;;&
		https://cloud-images.ubuntu.com/minimal/releases/*/release-*/*) use_latest=1 ;;&
		https://cloud-images.ubuntu.com/minimal/releases/*) flavor=${flavor:-minimal} ;;
		https://cloud-images.ubuntu.com/releases/*/release/*) use_latest=0 ;;&
		https://cloud-images.ubuntu.com/releases/*/release-*/*) use_latest=1 ;;&
		https://cloud-images.ubuntu.com/releases/*) flavor=${flavor:-server} ;;
		*)
			# echo "Unsupported image location: ${location}" >&2
			continue
			;;
		esac

		location_basename=$(basename "${location}")
		version=${version:-$(echo "${location_basename}" | cut -d- -f2)}
		flavor=${flavor:-$(echo "${location_basename}" | cut -d- -f3)}
		arch=$(echo "${location_basename}" | cut -d- -f5 | cut -d. -f1)
		path_suffix="${location_basename##*"${arch}"}"
		limayaml_arch=$(limayaml_arch "${arch}")
		if [[ ${use_latest} -eq 1 ]]; then
			latest_cache_key=${version}-${flavor}-${arch}-${path_suffix}
			location_digest=$(
				# shellcheck disable=SC2015
				[[ -v ubuntu_image_url_latest_cache[${latest_cache_key}] ]] && echo "${ubuntu_image_url_latest_cache[${latest_cache_key}]}" ||
					ubuntu_image_url_latest "${version}" "${flavor}" "${arch}" "${path_suffix}"
			)
			ubuntu_image_url_latest_cache[${latest_cache_key}]="${location_digest}"
			read -r location digest <<<"${location_digest}"
			if [[ -z ${location} ]]; then
				echo "Failed to get the latest image location for ${location_basename}" >&2
				continue
			elif [[ ${location} == "${location_before}" ]]; then
				continue
			fi
			image_entry="{\"location\": \"${location}\", \"arch\": \"${limayaml_arch}\", \"digest\": \"${digest}\"}"
			echo -e "${location}\n${digest}"
			if [[ ${kernel_location} != "null" ]]; then
				kernel_info=$(ubuntu_kernel_info_for_image_url "${location}")
				IFS=$'\t' read -r kernel_location kernel_digest initrd_location initrd_digest <<<"${kernel_info}"
				if [[ -n ${kernel_location} ]]; then
					image_entry=$(jq ". + {kernel: {location: \"${kernel_location}\", digest: \"${kernel_digest}\"}}" <<<"${image_entry}")
					[[ ${kernel_cmdline} != "null" ]] && image_entry=$(jq ".kernel.cmdline = \"${kernel_cmdline}\"" <<<"${image_entry}")
					echo -e "${kernel_location}\n${kernel_digest}"
				fi
				if [[ -n ${initrd_location} ]]; then
					image_entry=$(jq ". + {initrd: {location: \"${initrd_location}\", digest: \"${initrd_digest}\"}}" <<<"${image_entry}")
					echo -e "${initrd_location}\n${initrd_digest}"
				fi
			fi
		else
			release_cache_key=${version}-${flavor}-${arch}-${path_suffix}
			location=$(
				# shellcheck disable=SC2015
				[[ -v ubuntu_image_url_release_cache[${release_cache_key}] ]] && echo "${ubuntu_image_url_release_cache[${release_cache_key}]}" ||
					ubuntu_image_url_release "${version}" "${flavor}" "${arch}" "${path_suffix}"
			)
			ubuntu_image_url_release_cache[${release_cache_key}]="${location}"
			if [[ -z ${location} ]]; then
				echo "Failed to get the release image location for ${location_basename}" >&2
				continue
			elif [[ ${location} == "${location_before}" ]]; then
				continue
			fi
			image_entry="{\"location\": \"${location}\", \"arch\": \"${limayaml_arch}\"}"
			echo "${location}"
			if [[ ${kernel_location} != "null" ]]; then
				kernel_info=$(ubuntu_kernel_info_for_image_url "${location}")
				IFS=$'\t' read -r kernel_location kernel_digest initrd_location initrd_digest <<<"${kernel_info}"
				if [[ -n ${kernel_location} ]]; then
					image_entry=$(jq ". + {kernel: {location: \"${kernel_location}\"}}" <<<"${image_entry}")
					[[ ${kernel_cmdline} != "null" ]] && image_entry=$(jq ".kernel.cmdline = \"${kernel_cmdline}\"" <<<"${image_entry}")
					echo "${kernel_location}"
				fi
				if [[ -n ${initrd_location} ]]; then
					image_entry=$(jq ". + {initrd: {location: \"${initrd_location}\"}}" <<<"${image_entry}")
					echo "${initrd_location}"
				fi
			fi
		fi
		limactl edit --log-level error --set "
			[(.images.[] | path)].[${index}] as \$path|
			setpath(\$path; ${image_entry})
			.images[${index}].[] style = \"double\"
		" "${template}"
	done
done
