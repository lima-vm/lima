#!/usr/bin/env bash
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
*)
	echo "Unsupported arch: ${arch}" >&2
	exit 1
	;;
esac

# 2. extract location by parsing template using arch
readonly yq_filter="
[
    .images | map(select(.arch == \"${arch}\")) | [.[].location]
]|flatten|.[]
"
parsed=$(yq eval "${yq_filter}" "${template}")

# 3. get the image location
while IFS= read -r line; do arr+=("${line}"); done <<<"${parsed}"
readonly locations=("${arr[@]}")
for ((i = 0; i < ${#locations[@]}; i++)); do
	[[ ${locations[i]} != "null" ]] || continue
	http_code=$(curl -sIL -w "%{http_code}" "${locations[i]}" -o /dev/null)
	if [[ ${http_code} -eq 200 ]]; then
		location=${locations[i]}
		index=${i}
		break
	fi
done

# 4. check the image location is supported
if [[ -z ${location} ]]; then
	echo "Failed to get the image location for ${template}" >&2
	exit 1
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
yq -i eval "
    [(.images.[] | select(.arch == \"${arch}\") | path)].[${index}] + \"kernel\" as \$path|
    setpath(\$path; { \"location\": \"${kernel_location}\", \"digest\": \"${kernel_digest}\", \"cmdline\": \"${cmdline}\" })
" "${template}"
yq -i eval "
    [(.images.[] | select(.arch == \"${arch}\") | path)].[${index}] + \"initrd\" as \$path|
    setpath(\$path ; { \"location\": \"${initrd_location}\", \"digest\": \"${initrd_digest}\" })
" "${template}"

# 7. output kernel_location, kernel_digest, cmdline, initrd_location, initrd_digest
readonly outputs=(kernel_location kernel_digest cmdline initrd_location initrd_digest)
for output in "${outputs[@]}"; do
	echo "${output}=${!output}"
done
