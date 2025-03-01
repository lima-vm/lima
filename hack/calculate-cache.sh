#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This script calculates the expected content size, actual cached size, and cache-keys used in caching method before and after
# implementation in https://github.com/lima-vm/lima/pull/2508
#
# Answer to the question in https://github.com/lima-vm/lima/pull/2508#discussion_r1699798651

scriptdir=$(dirname "${BASH_SOURCE[0]}")
# shellcheck source=./common.inc.sh
. "${scriptdir}/cache-common-inc.sh"

# usage: [DEBUG=1] ./hack/calculate-cache.sh
# DEBUG=1 will save the collected information in .calculate-cache-collected-info-{before,after}.yaml
#
# This script does:
# 1. extracts `runs_on` and `template` from workflow file (.github/workflows/test.yml)
# 2. check each template for image, kernel, initrd, and nerdctl
# 3. detect size of image, kernel, initrd, and nerdctl (responses from remote are cached for faster iteration)
#    save the response in .check_location-response-cache.yaml
# 4. print content size, actual cache size (if available), by cache key
#
# The major differences for reducing cache usage are as follows:
# - it is now cached `~/.cache/lima/download/by-url-sha256/$sha256` instead of caching `~/.cache/lima/download`
# - the cache keys are now based on the image, kernel, initrd, and nerdctl digest instead of the template file's hash
# - enables the use of cache regardless of the operating system used to execute CI.
#
# The script requires the following commands:
# - gh: GitHub CLI.
#   Using to get the cache information
# - jq: Command-line JSON processor
#   Parse the workflow file and print runs-on and template.
#   Parse output from gh cache list
#   Calculate the expected content size, actual cached size, and cache-keys used.
# - limactl: lima CLI.
#   Using to validate the template file for getting nerdctl location and digest.
# - sha256sum: Print or check SHA256 (256-bit) checksums
# - xxd: make a hexdump or do the reverse.
#   Using to simulate the 'hashFile()' function in the workflow.
# - yq: Command-line YAML processor.
#   Parse the template file for image and nerdctl location, digest, and size.
#   Parse the cache response file for the cache.
#   Convert the collected information to JSON.

set -u -o pipefail

required_commands=(gh jq limactl sha256sum xxd yq)
for cmd in "${required_commands[@]}"; do
	if ! command -v "${cmd}" &>/dev/null; then
		echo "${cmd} is required. Please install it" >&2
		exit 1
	fi
done

# current workflow uses x86_64 only
arch=x86_64

LIMA_HOME=$(mktemp -d)
export LIMA_HOME

# parse the workflow file and print runs-on and template
# e.g.
# ```console
# $ print_runs_on_template_from_workflow .github/workflows/test.yml
# macos-12        templates/default.yaml
# ubuntu-24.04    templates/alpine.yaml
# ubuntu-24.04    templates/debian.yaml
# ubuntu-24.04    templates/fedora.yaml
# ubuntu-24.04    templates/archlinux.yaml
# ubuntu-24.04    templates/opensuse.yaml
# ubuntu-24.04    templates/experimental/net-user-v2.yaml
# ubuntu-24.04    templates/experimental/9p.yaml
# ubuntu-24.04    templates/docker.yaml
# ubuntu-24.04    templates/../hack/test-templates/alpine-iso-9p-writable.yaml
# ubuntu-24.04    templates/../hack/test-templates/test-misc.yaml
# macos-12        templates/vmnet.yaml
# macos-12        https://raw.githubusercontent.com/lima-vm/lima/v0.15.1/examples/ubuntu-lts.yaml
# macos-13        templates/experimental/vz.yaml
# macos-13        templates/fedora.yaml
# ```
function print_runs_on_template_from_workflow() {
	yq -o=j "$1" | jq -r '
		"./.github/actions/setup_cache_for_template" as $action |
		"\\$\\{\\{\\s*(?<path>\\S*)\\s*\\}\\}" as $pattern |
		.jobs | map_values(select(.steps)|
			."runs-on" as $runs_on |
			{
				template: .steps | map_values(select(.uses == $action)) | first |.with.template,
				matrix: .strategy.matrix
			} | select(.template) |
			. + { path: .template | (if test($pattern) then sub(".*\($pattern).*";"\(.path)")|split(".") else null end) } |
			(
				.template as $template|
				if .path then
					getpath(.path)|map(. as $item|$template|sub($pattern;$item))
				else
					[$template]
				end
			) | map("\($runs_on)\t\(.)")

		) | flatten |.[]
	'
}

# returns the OS name from the runner equivalent to the expression `${{ runner.os }}` in the workflow
# e.g.
# ```console
# $ runner_os_from_runner "macos-12"
# macOS
# $ runner_os_from_runner "ubuntu-24.04"
# Linux
# ```
function runner_os_from_runner() {
	# shellcheck disable=SC2249
	case "$1" in
	macos*)
		echo macOS
		;;
	ubuntu*)
		echo Linux
		;;
	esac
}

# format first column to MiB
# e.g.
# ```console
# $ echo 585498624 | size_to_mib
#   558.38 MiB
# ```
function size_to_mib() {
	awk '
		function mib(size) { return sprintf("%7.2f MiB", size / 1024 / 1024) }
		int($1)>0{ $1=" "mib($1) }
		int($2)>0{ $2=mib($2) }
		int($2)==0 && NF>1{ $2="<<missing>>" }
		{ print }
	'
}

# actual_cache_sizes=$(gh cache list --json key,createdAt,sizeInBytes|jq '[.[]|{"key":.key,"value":.sizeInBytes}]|from_entries')
# e.g.
# ```console
# $ echo "${actual_cache_sizes}"
# {
#   "Linux-1c3b2791d52735d916dc44767c745c2319eb7cae74af71bbf45ddb268f42fc1d": 810758533,
#   "Linux-231c66957fc2cdb18ea10e63f60770049026e29051ecd6598fc390b60d6a4fa6": 633036717,
#   "Linux-3b906d46fa532e3bc348c35fc8e7ede6c69f0b27032046ee2cbb56d4022d1146": 574242367,
#   "Linux-69a547b760dbf1650007ed541408474237bc611704077214adcac292de556444": 70310855,
#   "Linux-7782f8b4ff8cd378377eb79f8d61c9559b94bbd0c11d19eb380ee7bda19af04e": 494141177,
#   "Linux-8812aedfe81b4456d421645928b493b1f2f88aff04b7f3171207492fd44cd189": 812730766,
#   "Linux-caa7d8af214d55ad8902e82d5918e61573f3d6795d2b5ad9a35305e26fa0e6a9": 754723892,
#   "Linux-colima-v0.6.5": 226350335,
#   "Linux-de83bce0608d787e3c68c7a31c5fab2b6d054320fd7bf633a031845e2ee03414": 810691197,
#   "Linux-eb88a19dfcf2fb98278e7c7e941c143737c6d7cd8950a88f58e04b4ee7cef1bc": 570625794,
#   "Linux-f88f0b3b678ff6432386a42bdd27661133c84a36ad29f393da407c871b0143eb": 68490954,
#   "golangci-lint.cache-Linux-2850-74615231540133417fd618c72e37be92c5d3b3ad": 2434144,
#   "macOS-231c66957fc2cdb18ea10e63f60770049026e29051ecd6598fc390b60d6a4fa6": 633020464,
#   "macOS-49aa50a4872ded07ebf657c0eaf9e44ecc0c174d033a97c537ecd270f35b462f": 813179462,
#   "macOS-8f37f663956af5f743f0f99ab973729b6a02f200ebfac7a3a036eff296550732": 810756770,
#   "macOS-ef5509b5d4495c8c3590442ee912ad1c9a33f872dc4a29421c524fc1e2103b59": 813179476,
#   "macOS-upgrade-v0.15.1": 1157814690,
#   "setup-go-Linux-ubuntu20-go-1.23.0-02756877dbcc9669bb904e42e894c63aa9801138db94426a90a2d554f2705c52": 1015518352,
#   "setup-go-Linux-ubuntu20-go-1.23.0-6bce2eefc6111ace836de8bb322432c072805737d5f3c5a3d47d2207a05f50df": 936433302,
#   "setup-go-Linux-ubuntu24-go-1.22.6-02756877dbcc9669bb904e42e894c63aa9801138db94426a90a2d554f2705c52": 1090001859,
#   "setup-go-Linux-ubuntu24-go-1.23.0-02756877dbcc9669bb904e42e894c63aa9801138db94426a90a2d554f2705c52": 526146768,
#   "setup-go-Windows-go-1.23.0-02756877dbcc9669bb904e42e894c63aa9801138db94426a90a2d554f2705c52": 1155374040,
#   "setup-go-Windows-go-1.23.0-6bce2eefc6111ace836de8bb322432c072805737d5f3c5a3d47d2207a05f50df": 1056433137,
#   "setup-go-macOS-go-1.23.0-02756877dbcc9669bb904e42e894c63aa9801138db94426a90a2d554f2705c52": 1060919942,
#   "setup-go-macOS-go-1.23.0-6bce2eefc6111ace836de8bb322432c072805737d5f3c5a3d47d2207a05f50df": 982139209
# }
actual_cache_sizes=$(
	gh cache list --json key,createdAt,sizeInBytes \
		--jq 'sort_by(.createdAt)|reverse|unique_by(.key)|sort_by(.key)|map({"key":.key,"value":.sizeInBytes})|from_entries'
)

workflows=(
	.github/workflows/test.yml
)

# shellcheck disable=SC2016
echo "=> compare expected content size, actual cached size, and cache-keys used before and after the change in https://github.com/lima-vm/lima/pull/2508"
# iterate over before and after
for cache_method in before after; do
	echo "==> ${cache_method}"
	echo "content-size actual-size cache-key"
	output_yaml=$(
		for workflow in "${workflows[@]}"; do
			print_runs_on_template_from_workflow "${workflow}"
		done | while IFS=$'\t' read -r runner template; do
			template=$(download_template_if_needed "${template}") || continue
			arch=$(detect_arch "${template}" "${arch}") || continue
			index=$(print_image_locations_for_arch_from_template "${template}" "${arch}" | print_valid_image_index) || continue
			image_kernel_initrd_info=$(print_image_kernel_initrd_locations_with_digest_for_arch_from_template_at_index "${template}" "${index}" "${arch}") || continue
			# shellcheck disable=SC2034 # shellcheck does not detect dynamic variables usage
			read -r image_location image_digest kernel_location kernel_digest initrd_location initrd_digest <<<"${image_kernel_initrd_info}"
			containerd_info=$(print_containerd_config_for_arch_from_template "${template}" "${@:2}") || continue
			# shellcheck disable=SC2034 # shellcheck does not detect dynamic variables usage
			read -r _containerd_enabled containerd_location containerd_digest <<<"${containerd_info}"

			if [[ ${cache_method} != after ]]; then
				key=$(runner_os_from_runner "${runner}" || true)-$(hash_file "${template}")
			else
				key=$(cache_key_from_prefix_location_and_digest image "${image_location}" "${image_digest}")
			fi
			size=$(size_from_location "${image_location}")
			for prefix in containerd kernel initrd; do
				location="${prefix}_location"
				digest="${prefix}_digest"
				[[ ${!location} != null ]] || continue
				if [[ ${cache_method} != after ]]; then
					# previous caching method packages all files in download to a single cache key
					size=$((size + $(size_from_location "${!location}")))
				else
					# new caching method caches each file separately
					key_for_prefix=$(cache_key_from_prefix_location_and_digest "${prefix}" "${!location}" "${!digest}")
					size_for_prefix=$(size_from_location "${!location}")
					printf -- "- key: %s\n  template: %s\n  location: %s\n  digest: %s\n  size: %s\n" \
						"${key_for_prefix}" "${template}" "${!location}" "${!digest}" "${size_for_prefix}"
				fi
			done
			printf -- "- key: %s\n  template: %s\n  location: %s\n  digest: %s\n  size: %s\n" \
				"${key}" "${template}" "${image_location}" "${image_digest}" "${size}"
		done
	)
	output_json=$(yq -o=j . <<<"${output_yaml}")

	# print size key
	jq --argjson actual_size "${actual_cache_sizes}" -r 'unique_by(.key)|sort_by(.key)|.[]|[.size, $actual_size[.key] // 0, .key]|@tsv' <<<"${output_json}" | size_to_mib
	# total
	echo "------------"
	jq '[unique_by(.key)|.[]|.size]|add' <<<"${output_json}" | size_to_mib
	# save the collected information as yaml if DEBUG is set
	if [[ -n ${DEBUG:+1} ]]; then
		cat <<<"${output_yaml}" >".calculate-cache-collected-info-${cache_method}.yaml"
		echo "Saved the collected information in .calculate-cache-collected-info-${cache_method}.yaml"
	fi
	echo ""
done
