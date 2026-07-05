#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

[ "${LIMA_CIDATA_MOUNTS:-0}" -gt 0 ] || exit 0
[ "$LIMA_CIDATA_VMTYPE" = "wsl2" ] || exit 0

INFO() {
	echo "LIMA $(date -Iseconds)| mounts: $*"
}

WARNING() {
	echo "LIMA $(date -Iseconds)| mounts: WARNING: $*" >&2
}

# shellcheck disable=SC1003
BACKSLASH='\'

ro_mounts=""

is_path_ro() {
	check_path="$1"
	for ro_entry in $ro_mounts; do
		case "$check_path/" in
		"$ro_entry/"*) return 0 ;;
		esac
	done
	if command -v findmnt >/dev/null 2>&1; then
		if findmnt -n -O ro -T "$check_path" >/dev/null 2>&1; then
			return 0
		fi
	fi
	return 1
}

is_exact_ro_mount() {
	check_path="$1"
	if command -v findmnt >/dev/null 2>&1; then
		if findmnt -n -O ro -M "$check_path" >/dev/null 2>&1; then
			return 0
		fi
	fi
	return 1
}

for i in $(seq 0 $((LIMA_CIDATA_MOUNTS - 1))); do
	location=""
	mountpoint=""
	writable=0
	eval "location=\${LIMA_CIDATA_MOUNTS_${i}_LOCATION}"
	eval "mountpoint=\${LIMA_CIDATA_MOUNTS_${i}_MOUNTPOINT}"
	eval "writable=\${LIMA_CIDATA_MOUNTS_${i}_WRITABLE}"

	if [ -z "$location" ] || [ -z "$mountpoint" ]; then
		continue
	fi

	# Step 1: Translate Windows Host Path to WSL path
	wsl_path=$(wslpath -u "$location" 2>/dev/null)
	if [ -z "$wsl_path" ]; then
		cleaned_loc="$location"
		while [ ${#cleaned_loc} -gt 2 ] && { [ "${cleaned_loc#"${cleaned_loc%?}"}" = "/" ] || [ "${cleaned_loc#"${cleaned_loc%?}"}" = "$BACKSLASH" ]; }; do
			cleaned_loc="${cleaned_loc%?}"
		done
		case "$cleaned_loc" in
		[A-Za-z]:*)
			drive=$(printf "%s" "$cleaned_loc" | cut -c1 | tr '[:upper:]' '[:lower:]')
			rest=$(printf "%s" "$cleaned_loc" | cut -c3- | tr "$BACKSLASH" '/')
			wsl_path="/mnt/${drive}${rest}"
			;;
		*)
			WARNING "Cannot translate host path $location to WSL path; skipping mount"
			continue
			;;
		esac
	fi

	# If the mountPoint is the same as the WSL path, it is already natively accessible via WSL automount
	if [ "$wsl_path" = "$mountpoint" ]; then
		if [ "$writable" = "0" ]; then
			if is_exact_ro_mount "$mountpoint"; then
				INFO "$mountpoint is already mounted read-only, skipping"
			else
				# If the user requested read-only, we must bind-mount it onto itself and remount as read-only
				# to enforce writable:false without affecting the parent native mount.
				INFO "Enforcing read-only on native automount at $mountpoint"
				if mount --bind "$mountpoint" "$mountpoint" 2>/dev/null && mount -o remount,ro,bind "$mountpoint" 2>/dev/null; then
					ro_mounts="$ro_mounts $mountpoint"
				else
					WARNING "Path $location is natively read-write; cannot enforce writable:false on $mountpoint"
					umount "$mountpoint" 2>/dev/null || true
				fi
			fi
		else
			if is_path_ro "$mountpoint"; then
				INFO "Enforcing read-write on nested native automount at $mountpoint"
				if ! mount --bind "$mountpoint" "$mountpoint" 2>/dev/null || ! mount -o remount,rw,bind "$mountpoint" 2>/dev/null; then
					WARNING "Path $location is in a read-only mount; cannot enforce writable:true on $mountpoint"
					umount "$mountpoint" 2>/dev/null || true
				fi
			else
				INFO "Path $location is already available at $mountpoint via WSL automount"
			fi
		fi
		continue
	fi

	# Step 2: Idempotent Mount Check
	if mountpoint -q "$mountpoint"; then
		INFO "$mountpoint is already mounted, skipping"
	else
		mkdir -p "$mountpoint"
		INFO "Mounting $wsl_path to $mountpoint (writable: $writable)"
		if mount --bind "$wsl_path" "$mountpoint"; then
			if [ "$writable" = "0" ]; then
				if ! mount -o remount,ro,bind "$mountpoint"; then
					WARNING "Failed to remount $mountpoint as read-only; path remains read-write"
				else
					ro_mounts="$ro_mounts $mountpoint"
					INFO "Successfully bind-mounted $wsl_path to $mountpoint (read-only)"
				fi
			else
				if is_path_ro "$wsl_path" || is_path_ro "$mountpoint"; then
					INFO "Enforcing read-write on nested bind mount at $mountpoint"
					mount -o remount,rw,bind "$mountpoint" 2>/dev/null || WARNING "Failed to remount $mountpoint as read-write"
				fi
				INFO "Successfully bind-mounted $wsl_path to $mountpoint"
			fi
		else
			WARNING "Failed to bind mount $wsl_path to $mountpoint. Falling back to symlink. Note: symlink fallback is a degraded mode — tools relying on mount-point detection (df, container engines doing bind-mount-of-bind-mount) will not treat a symlinked path as a mount boundary."
			# Cleanup leaf directory and create symlink
			# Note: intermediate parent directories created by mkdir -p are left behind (harmless in practice)
			rmdir "$mountpoint" 2>/dev/null || true
			if [ ! -e "$mountpoint" ]; then
				ln -sfn "$wsl_path" "$mountpoint"
				if [ "$writable" = "0" ]; then
					WARNING "Fell back to symlink for $mountpoint, but note that symlinks CANNOT enforce read-only status. Target is writable!"
				fi
				INFO "Successfully symlinked $wsl_path to $mountpoint"
			else
				WARNING "Cannot create symlink at $mountpoint: file or non-empty directory exists"
			fi
		fi
	fi
done
