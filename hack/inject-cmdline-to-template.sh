#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

#
# This script does
# 1. detect arch from template if not provided
# 2. extract location by parsing template using arch
# 3. get the image location
# 4. check the image location is supported
# 5. build the kernel and initrd location, digest, and cmdline
# 6. inject the kernel and initrd location, digest, and cmdline to the template
# 7. output kernel_location, kernel_digest, cmdline, initrd_location, initrd_digest

set -eu -o pipefail

template="$1"
appending_options="$2"
# 1. detect arch from template if not provided
arch="${3:-$(yq '.arch // ""' "${template}")}"
arch="${arch:-$(uname -m)}"

# normalize arch. amd64 -> x86_64, arm64 -> aarch64
case "${arch}" in
amd64 | x86_64) arch=x86_64 ;;
aarch64 | arm64) arch=aarch64 ;;
armv7l | armhf) arch=armv7l ;;
riscv64) arch=riscv64 ;;
*)
	echo "Unsupported arch: ${arch}" >&2
	exit 1
	;;
esac

# 2. extract location by parsing template using arch
readonly yq_filter="
    .images[]|select(.arch == \"${arch}\")|.location
"
parsed=$(yq eval "${yq_filter}" "${template}")

# 3. get the image location
function check_location() {
	local location=$1 http_code
	http_code=$(curl -sIL -w "%{http_code}" "${location}" -o /dev/null)
	[[ ${http_code} -eq 200 ]]
}
while IFS= read -r line; do arr+=("${line}"); done <<<"${parsed}"
readonly locations=("${arr[@]}")
for ((i = 0; i < ${#locations[@]}; i++)); do
	[[ ${locations[i]} != "null" ]] || continue
	# shellcheck disable=SC2310
	if check_location "${locations[i]}"; then
		location=${locations[i]}
		index=${i}
		break
	fi
done

# 4. check the image location is supported
if [[ -z ${location} ]]; then
	echo "Failed to get the image location for ${template}" >&2
	exit 1
elif [[ ${location} == https://cloud-images.ubuntu.com/minimal/* ]]; then
	readonly default_cmdline="root=/dev/vda1 ro console=tty1 console=ttyAMA0"
elif [[ ${location} == https://cloud-images.ubuntu.com/* ]]; then
	readonly default_cmdline="root=LABEL=cloudimg-rootfs ro console=tty1 console=ttyAMA0"
else
	echo "Unsupported image location: ${location}" >&2
	exit 1
fi

# 5. build the kernel and initrd location, digest, and cmdline
location_dirname=$(dirname "${location}")/unpacked
sha256sums=$(curl -sSLf "${location_dirname}/SHA256SUMS")
location_basename=$(basename "${location}")

# cmdline
cmdline="${default_cmdline} ${appending_options}"

# kernel
kernel_basename="${location_basename/.img/-vmlinuz-generic}"
kernel_digest=$(awk "/${kernel_basename}/{print \"sha256:\"\$1}" <<<"${sha256sums}")
kernel_location="${location_dirname}/${kernel_basename}"

# initrd
initrd_basename="${location_basename/.img/-initrd-generic}"
initrd_digest=$(awk "/${initrd_basename}/{print \"sha256:\"\$1}" <<<"${sha256sums}")
initrd_location="${location_dirname}/${initrd_basename}"

# 6. inject the kernel and initrd location, digest, and cmdline to the template
function inject_to() {
	# shellcheck disable=SC2034
	local template=$1 arch=$2 index=$3 key=$4 location=$5 digest=$6 cmdline=${7:-} fields=() IFS=,
	# shellcheck disable=SC2310
	check_location "${location}" || return 0
	for field_name in location digest cmdline; do
		[[ -z ${!field_name} ]] || fields+=("\"${field_name}\": \"${!field_name}\"")
	done
	limactl edit --log-level error --set "setpath([(.images[] | select(.arch == \"${arch}\") | path)].[${index}] + \"${key}\"; { ${fields[*]}})" "${template}"
}
inject_to "${template}" "${arch}" "${index}" "kernel" "${kernel_location}" "${kernel_digest}" "${cmdline}"
inject_to "${template}" "${arch}" "${index}" "initrd" "${initrd_location}" "${initrd_digest}"

# 7. output kernel_location, kernel_digest, cmdline, initrd_location, initrd_digest
readonly outputs=(kernel_location kernel_digest cmdline initrd_location initrd_digest)
for output in "${outputs[@]}"; do
	echo "${output}=${!output}"
done
