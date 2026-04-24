# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

# End-to-end flow:
# 1. Create a tiny host RAM disk and attach it to a VZ VM as either /dev/diskN or /dev/rdiskN.
# 2. In the guest, partition it as GPT with an msftdata partition, format it as exFAT, and write test data.
# 3. Stop the VM, mount the same partition on the macOS host, verify the guest write, and append more data.
# 4. Start the VM again, verify the host write from the guest, append again, stop the VM, and verify the final
#    contents from the host once more. This exercises the real guest<->host handoff instead of just local helpers.

configured_block_device=""
host_disk_device=""
host_partition=""
instance_name=""

teardown() {
	if [[ -n ${instance_name} ]]; then
		limactl unprotect "${instance_name}" || :
		limactl delete --force "${instance_name}" || :
	fi
	if [[ -n ${host_partition} ]]; then
		sudo diskutil unmount force "${host_partition}" >/dev/null 2>&1 || :
	fi
	if [[ -n ${host_disk_device} ]]; then
		sudo diskutil unmountDisk force "${host_disk_device}" >/dev/null 2>&1 || :
		sudo hdiutil detach "${host_disk_device}" -force >/dev/null 2>&1 || :
	fi
	configured_block_device=""
	host_disk_device=""
	host_partition=""
	instance_name=""
}

bats::on_failure() {
	if [[ -n ${instance_name} ]] && [[ -d ${LIMA_HOME}/${instance_name} ]]; then
		tail -n 100 "${LIMA_HOME}/${instance_name}"/*.log || :
		limactl shell "${instance_name}" -- sudo cat /var/log/cloud-init-output.log || :
	fi
}

create_ramdisk() {
	host_disk_device=$(sudo hdiutil attach -nomount ram://65536 | awk 'NR==1 {print $1}')
	[[ -n ${host_disk_device} ]]
}

configure_block_device_path() {
	case "$1" in
	disk)
		configured_block_device="${host_disk_device}"
		;;
	rdisk)
		configured_block_device="${host_disk_device/\/dev\/disk/\/dev\/rdisk}"
		;;
	*)
		echo "unknown block device kind: $1" >&2
		return 1
		;;
	esac
}

start_vz_instance_with_cli_flag() {
	local extra_args=()
	if [[ -n ${LIMACTL_CREATE_ARGS:-} ]]; then
		read -r -a extra_args <<<"${LIMACTL_CREATE_ARGS}"
	fi

	limactl delete --force "${instance_name}" || :
	limactl start --tty=false \
		"${extra_args[@]}" \
		--vm-type=vz \
		--block-device "${configured_block_device}" \
		--name "${instance_name}" \
		template:default \
		3>&- 4>&-
}

start_vz_instance_with_yaml() {
	local config_file="${BATS_TEST_TMPDIR}/${instance_name}.yaml"
	local extra_args=()
	if [[ -n ${LIMACTL_CREATE_ARGS:-} ]]; then
		read -r -a extra_args <<<"${LIMACTL_CREATE_ARGS}"
	fi

	cat >"${config_file}" <<EOF
base: template:default
vmType: vz
blockDevices:
  - ${configured_block_device}
EOF

	limactl delete --force "${instance_name}" || :
	limactl start --tty=false \
		"${extra_args[@]}" \
		--name "${instance_name}" \
		"${config_file}" \
		3>&- 4>&-
}

format_and_write_from_guest() {
	local guest_block_device=$1
	local guest_partition_by_id=$2
	local volume_label=$3
	local expected_contents=$4

	limactl shell "${instance_name}" bash -eus -- "${guest_block_device}" "${guest_partition_by_id}" "${volume_label}" "${expected_contents}" <<'EOF'
guest_block_device="$1"
guest_partition_by_id="$2"
volume_label="$3"
expected_contents="$4"
guest_partition="$guest_partition_by_id"

if ! command -v mkfs.exfat >/dev/null 2>&1 || ! command -v parted >/dev/null 2>&1; then
    sudo apt-get update
    sudo apt-get install -y exfatprogs parted
fi

# macOS only auto-recognizes the guest-created exFAT filesystem after shutdown if the GPT partition type is
# Microsoft Basic Data rather than the generic Linux filesystem type.
sudo parted -s "${guest_block_device}" -- mklabel gpt mkpart primary 1MiB 100% set 1 msftdata on
sudo udevadm settle
if [ ! -e "${guest_partition}" ]; then
    # /dev/disk/by-id/...-part1 is the stable path we expect, but udev can lag behind partition creation.
    # Fall back to the first partition discovered directly from lsblk so the test still validates the same disk.
    guest_partition="$(lsblk -nrpo PATH "${guest_block_device}" | sed -n '2p')"
fi
test -n "${guest_partition}"
sudo mkfs.exfat -n "${volume_label}" "${guest_partition}"
sudo mkdir -p /mnt/test
sudo mount "${guest_partition}" /mnt/test
printf '%s\n' "${expected_contents}" | sudo tee /mnt/test/test.txt >/dev/null
sudo sync
if [ "$(cat /mnt/test/test.txt)" != "${expected_contents}" ]; then
    exit 1
fi
sudo umount /mnt/test
EOF
}

append_and_verify_from_guest() {
	local guest_block_device=$1
	local guest_partition_by_id=$2
	local append_line=$3
	local expected_before=$4
	local expected_after=$5

	limactl shell "${instance_name}" bash -eus -- "${guest_block_device}" "${guest_partition_by_id}" "${append_line}" "${expected_before}" "${expected_after}" <<'EOF'
guest_block_device="$1"
guest_partition_by_id="$2"
append_line="$3"
expected_before="$4"
expected_after="$5"
guest_partition="$guest_partition_by_id"

if [ ! -e "${guest_partition}" ]; then
    # After the host mount/unmount handoff, the by-id partition symlink may reappear later than the block device.
    guest_partition="$(lsblk -nrpo PATH "${guest_block_device}" | sed -n '2p')"
fi
test -n "${guest_partition}"
sudo mkdir -p /mnt/test
sudo mount "${guest_partition}" /mnt/test
if [ "$(cat /mnt/test/test.txt)" != "${expected_before}" ]; then
    exit 1
fi
printf '%s\n' "${append_line}" | sudo tee -a /mnt/test/test.txt >/dev/null
sudo sync
if [ "$(cat /mnt/test/test.txt)" != "${expected_after}" ]; then
    exit 1
fi
sudo umount /mnt/test
EOF
}

mount_and_verify_from_host() {
	local expected_contents=$1

	# Resolve the host-visible partition that backs the same raw RAM disk after the guest has partitioned it.
	host_partition="$(diskutil list "${host_disk_device}" | awk '$NF ~ /^disk[0-9]+s[0-9]+$/ {print "/dev/" $NF; exit}')"
	[[ -n ${host_partition} ]]

	sudo diskutil mount "${host_partition}" >/dev/null
	local host_mount_point
	host_mount_point=$(diskutil info -plist "${host_partition}" | plutil -extract MountPoint raw -o - -)
	[[ -n ${host_mount_point} ]]

	run -0 cat "${host_mount_point}/test.txt"
	assert_output "${expected_contents}"
}

append_and_verify_from_host() {
	local append_line=$1
	local expected_before=$2
	local expected_after=$3

	mount_and_verify_from_host "${expected_before}"

	local host_mount_point
	host_mount_point=$(diskutil info -plist "${host_partition}" | plutil -extract MountPoint raw -o - -)
	[[ -n ${host_mount_point} ]]

	printf '%s\n' "${append_line}" | sudo tee -a "${host_mount_point}/test.txt" >/dev/null
	run -0 cat "${host_mount_point}/test.txt"
	assert_output "${expected_after}"
}

unmount_host_partition() {
	[[ -n ${host_partition} ]]
	sudo diskutil unmount force "${host_partition}"
	host_partition=""
}

run_roundtrip_test() {
	local block_device_kind=$1
	local start_mode=$2
	local expected_after_guest_1
	local expected_after_host_1
	local expected_after_guest_2
	local block_device_id
	local guest_block_device
	local guest_partition_by_id
	local volume_label

	[[ $(uname) == "Darwin" ]] || skip "requires macOS host"
	command -v hdiutil >/dev/null
	command -v diskutil >/dev/null
	command -v plutil >/dev/null

	instance_name="vz-block-device-${block_device_kind}"
	create_ramdisk
	configure_block_device_path "${block_device_kind}"

	block_device_id=$(basename "${configured_block_device}")
	guest_block_device="/dev/disk/by-id/virtio-${block_device_id}"
	guest_partition_by_id="/dev/disk/by-id/virtio-${block_device_id}-part1"
	volume_label="LIMABD${RANDOM}"
	expected_after_guest_1="$(printf 'guest-1')"
	expected_after_host_1="$(printf 'guest-1\nhost-1')"
	expected_after_guest_2="$(printf 'guest-1\nhost-1\nguest-2')"

	# Cover both user-facing entry points: CLI flag wiring and YAML config wiring.
	case "${start_mode}" in
	cli)
		start_vz_instance_with_cli_flag
		;;
	yaml)
		start_vz_instance_with_yaml
		;;
	*)
		echo "unknown start mode: ${start_mode}" >&2
		return 1
		;;
	esac
	format_and_write_from_guest "${guest_block_device}" "${guest_partition_by_id}" "${volume_label}" "${expected_after_guest_1}"
	limactl stop "${instance_name}"

	# The host and guest must not mount the filesystem concurrently; this validates the intended stop/remount handoff.
	append_and_verify_from_host "host-1" "${expected_after_guest_1}" "${expected_after_host_1}"
	unmount_host_partition

	limactl start --tty=false "${instance_name}" 3>&- 4>&-
	append_and_verify_from_guest "${guest_block_device}" "${guest_partition_by_id}" "guest-2" "${expected_after_host_1}" "${expected_after_guest_2}"
	limactl stop "${instance_name}"

	mount_and_verify_from_host "${expected_after_guest_2}"
}

@test "VZ block device roundtrip via CLI flag and /dev/diskN" {
	# /dev/diskN is the documented host-facing path users are most likely to provide.
	run_roundtrip_test disk cli
}

@test "VZ block device roundtrip via YAML and /dev/rdiskN" {
	# /dev/rdiskN exercises the raw-device variant and the YAML blockDevices configuration path in one case.
	run_roundtrip_test rdisk yaml
}
