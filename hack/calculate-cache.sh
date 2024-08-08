#!/usr/bin/env bash
# This script calculates the expected content size, actual cached size, and cache-keys used in caching method prior and after
# implementation in https://github.com/lima-vm/lima/pull/2508
#
# Answer to the question in https://github.com/lima-vm/lima/pull/2508#discussion_r1699798651
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

# shellcheck disable=SC2034
macos_12=(
	# integration
	examples/default.yaml
	# vmnet
	examples/vmnet.yaml
	# upgrade
	https://raw.githubusercontent.com/lima-vm/lima/v0.15.1/examples/ubuntu-lts.yaml
)

# shellcheck disable=SC2034
ubuntu_2204=(
	# integration-linux
	examples/alpine.yaml
	examples/debian.yaml
	examples/fedora.yaml
	examples/archlinux.yaml
	examples/opensuse.yaml
	examples/experimental/net-user-v2.yaml
	examples/experimental/9p.yaml
	examples/docker.yaml
	examples/../hack/test-templates/alpine-9p-writable.yaml
	examples/../hack/test-templates/test-misc.yaml
)

# shellcheck disable=SC2034
macos_13=(
	# vz
	examples/experimental/vz.yaml
	examples/fedora.yaml
)

runners=(
	macos_12
	ubuntu_2204
	macos_13
)

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

function check_location() {
	location="$1"
	readonly cache_file="./.calculate-cache-response-cache.yaml"
	# check response_cache.yaml for the cache
	if [[ -f ${cache_file} ]]; then
		cached=$(yq -e eval ".[\"${location}\"]" "${cache_file}" 2>/dev/null) && echo "${cached}" && return
	else
		touch "${cache_file}"
	fi
	http_code_and_size=$(curl -sIL -w "%{http_code} %header{Content-Length}" "${location}" -o /dev/null)
	yq eval ".[\"${location}\"] = \"${http_code_and_size}\"" -i "${cache_file}"
	echo "${http_code_and_size}"
}

function print_location_digest_size_hash_from_template() {
	readonly template=$1
	case "${template}" in
	http*)
		template_yaml=$(curl -sSL "${template}")
		;;
	*)
		template_yaml=$(<"${template}")
		;;
	esac
	readonly yq_filter="
		[
			.images | map(select(.arch == \"${arch}\")) | [.[0,1].location, .[0,1].digest],
			.containerd|[.system or .user],
			.containerd.archives | map(select(.arch == \"${arch}\")) | [.[0].location, .[0].digest]
		]|flatten|.[]
	"
	if command -v limactl &>/dev/null; then
		parsed=$(limactl validate <(echo "${template_yaml}") --fill 2>/dev/null | yq eval "${yq_filter}")
	else
		parsed=$(yq eval "${yq_filter}" <<<"${template_yaml}")
	fi
	# macOS earlier than 15.0 uses bash 3.2.57, which does not support readarray -t
	# readarray -t arr <<<"${parsed}"
	while IFS= read -r line; do arr+=("${line}"); done <<<"${parsed}"
	readonly locations=("${arr[@]:0:2}") digests=("${arr[@]:2:2}")
	readonly containerd="${arr[4]}" containerd_location="${arr[5]}" containerd_digest="${arr[6]}"
	declare location digest size hash
	for ((i = 0; i < ${#locations[@]}; i++)); do
		[[ ${locations[i]} != null ]] || continue
		http_code_and_size=$(check_location "${locations[i]}")
		read -r http_code size <<<"${http_code_and_size}"
		if [[ ${http_code} -eq 200 ]]; then
			location=${locations[i]}
			digest=${digests[i]}
			break
		fi
	done
	if [[ -z ${location} ]]; then
		echo "Failed to get the image location for ${template}" >&2
		return 1
	fi
	hash=$(sha256sum <<<"${template_yaml}" | cut -d' ' -f1 | xxd -r -p | sha256sum | cut -d' ' -f1)
	declare containerd_size
	containerd_http_code_and_size=$(check_location "${containerd_location}")
	read -r _containerd_http_code containerd_size <<<"${containerd_http_code_and_size}"
	echo "${location} ${digest} ${size} ${hash} ${containerd} ${containerd_location} ${containerd_digest} ${containerd_size}"
}

# format first column to MiB
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
actual_cache_sizes=$(
	gh cache list --json key,createdAt,sizeInBytes |
		jq 'sort_by(.createdAt)|reverse|unique_by(.key)|sort_by(.key)|map({"key":.key,"value":.sizeInBytes})|from_entries'
)

# shellcheck disable=SC2016
for cache_method in prior after; do
	echo "==> expected content size, actual cached size, and cache-keys used in caching method ${cache_method} implementation in https://github.com/lima-vm/lima/pull/2508"
	echo "content-size actual-size cache-key"
	output_yaml=$(
		for runner in "${runners[@]}"; do
			runner_os=$(runner_os_from_runner "${runner}")
			declare -n ref="${runner}"
			tepmlates_used_in_test_yml=("${ref[@]}")
			for template in "${tepmlates_used_in_test_yml[@]}"; do
				location_digest_size_hash=$(print_location_digest_size_hash_from_template "${template}") || continue
				read -r location digest size hash containerd containerd_location containerd_digest containerd_size <<<"${location_digest_size_hash}"
				if [[ ${cache_method} == prior ]]; then
					key=${runner_os}-${hash}
				elif [[ ${digest} == null ]]; then
					key=image:$(basename "${location}")-url-sha256:$(echo -n "${location}" | sha256sum | cut -d' ' -f1)
				else
					key=image:$(basename "${location}")-${digest}
				fi
				if [[ ${containerd} == true ]]; then
					if [[ ${cache_method} == prior ]]; then
						# previous caching method packages the containerd archive with the image
						size=$((size + containerd_size))
					else
						# new caching method packages the containerd archive separately
						containerd_key=containerd:$(basename "${containerd_location}")-${containerd_digest}
						cat <<-EOF
							- key: ${containerd_key}
							  template: ${template}
							  location: ${containerd_location}
							  digest: ${containerd_digest}
							  size: ${containerd_size}
						EOF
					fi
				fi
				cat <<-EOF
					- key: ${key}
					  template: ${template}
					  location: ${location}
					  digest: ${digest}
					  size: ${size}
				EOF
				# echo -e "- key: ${key}\n  template: ${template}\n  location: ${location}\n  size: ${size}\n  containerd: ${containerd}\n  containerd_location: ${containerd_location}\n  containerd_digest: ${containerd_digest}"
			done
		done
	)
	cat <<<"${output_yaml}" >".calculate-cache-collected-info-${cache_method}.yaml"
	output_json=$(yq -o=j . <<<"${output_yaml}" | tee ".calculate-cache-collected-info-${cache_method}.json")

	# print size key
	jq --argjson actual_size "${actual_cache_sizes}" -r 'unique_by(.key)|sort_by(.key)|.[]|[.size, $actual_size[.key] // 0, .key]|@tsv' <<<"${output_json}" | size_to_mib
	# total
	echo "------------"
	jq '[unique_by(.key)|.[]|.size]|add' <<<"${output_json}" | size_to_mib
	echo ""
done
