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
			wsl_path="$location"
			;;
		esac
	fi

	# If the mountPoint is the same as the WSL path, it is already natively accessible via WSL automount
	if [ "$wsl_path" = "$mountpoint" ]; then
		if [ "$writable" = "0" ]; then
			# If the user requested read-only, we must bind-mount it onto itself and remount as read-only
			# to enforce writable:false without affecting the parent native mount.
			INFO "Enforcing read-only on native automount at $mountpoint"
			if ! mount --bind "$mountpoint" "$mountpoint" 2>/dev/null || ! mount -o remount,ro,bind "$mountpoint" 2>/dev/null; then
				WARNING "Path $location is natively read-write; cannot enforce writable:false on $mountpoint"
			fi
		else
			INFO "Path $location is already available at $mountpoint via WSL automount"
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
					WARNING "Failed to remount $mountpoint as read-only. Unmounting for safety."
					umount "$mountpoint"
					# Cleanup leaf directory and create symlink fallback
					# Note: intermediate parent directories created by mkdir -p are left behind (harmless in practice)
					rmdir "$mountpoint" 2>/dev/null || true
					if [ ! -e "$mountpoint" ]; then
						ln -sfn "$wsl_path" "$mountpoint"
						WARNING "Fell back to symlink for $mountpoint, but note that symlinks CANNOT enforce read-only status. Target is writable!"
					fi
				fi
			fi
			INFO "Successfully bind-mounted $wsl_path to $mountpoint"
		else
			WARNING "Failed to bind mount $wsl_path to $mountpoint. Falling back to symlink. Note: symlink fallback is a degraded mode — tools relying on mount-point detection (df, container engines doing bind-mount-of-bind-mount) will not treat a symlinked path as a mount boundary."
			# Cleanup leaf directory and create symlink
			# Note: intermediate parent directories created by mkdir -p are left behind (harmless in practice)
			rmdir "$mountpoint" 2>/dev/null || true
			if [ ! -e "$mountpoint" ]; then
				ln -sfn "$wsl_path" "$mountpoint"
				INFO "Successfully symlinked $wsl_path to $mountpoint"
			else
				WARNING "Cannot create symlink at $mountpoint: file or non-empty directory exists"
			fi
		fi
	fi
done

# Step 3: Merge [automount] options = "metadata" into /etc/wsl.conf
if grep -q '^\[automount\]' /etc/wsl.conf 2>/dev/null; then
	# section exists — patch or insert the options= line within it
	if awk '/^\[automount\]/{f=1;next} /^\[/{f=0} f&&/^options/{print; exit}' /etc/wsl.conf | grep -q .; then
		sed -i '/^\[automount\]/,/^\[/ s|^options.*|options = "metadata"|' /etc/wsl.conf
	else
		sed -i '/^\[automount\]/a options = "metadata"' /etc/wsl.conf
	fi
else
	printf '\n[automount]\noptions = "metadata"\n' >>/etc/wsl.conf
fi
INFO "wsl.conf automount metadata options verified/merged"
