#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

set -eu

INFO() {
	echo "LIMA $(date -Iseconds)| $*"
}

WARNING() {
	echo "LIMA $(date -Iseconds)| WARNING: $*"
}

UNAME="$(uname -s)"

RUN="/run"
if [ "${UNAME}" != "Linux" ]; then
	RUN="/var/run"
fi
rm -f "${RUN}/lima-boot-done"

# shellcheck disable=SC2163
while read -r line; do [ -n "$line" ] && export "$line"; done <"${LIMA_CIDATA_MNT}"/lima.env
# shellcheck disable=SC2163
while read -r line; do [ -n "$line" ] && export "$line"; done <"${LIMA_CIDATA_MNT}"/param.env

# shellcheck disable=SC2163
while read -r line; do
	# pam_env implementation:
	# - '#' is treated the same as newline; terminates value
	# - skip leading tabs and spaces
	# - skip leading "export " prefix (only single space)
	# - skip leading quote ('\'' or '"') on the value side
	# - skip trailing quote only if leading quote has been skipped;
	#   quotes don't need to match; trailing quote may be omitted
	line="$(echo "$line" | sed -E "s/^[ \\t]*(export )?//; s/#.*//; s/(^[^=]+=)[\"'](.*[^\"'])?[\"']?$/\1\2/")"
	[ -n "$line" ] && export "$line"
done <"${LIMA_CIDATA_MNT}"/etc_environment

PATH="${LIMA_CIDATA_MNT}"/util:"${PATH}"
export PATH

CODE=0

# Don't make any changes to /etc or /var/lib until boot.Linux/04-persistent-data-volume.sh
# has run because it might move the directories to /mnt/data on first boot. In that
# case changes made on restart would be lost.

run_boot_scripts() {
	boot="$1"
	if [ -e "${boot}" ]; then
		for f in "${boot}"/*.sh; do
			INFO "Executing $f"
			if ! "$f"; then
				WARNING "Failed to execute $f"
				CODE=1
			fi
		done
	fi
}

# The boot.essential.${UNAME} scripts are executed in plain mode too.
run_boot_scripts "${LIMA_CIDATA_MNT}/boot.essential.${UNAME}"

if [ "$LIMA_CIDATA_PLAIN" = "1" ]; then
	INFO "Plain mode. Skipping to run non-essential boot scripts. Provisioning scripts will be still executed. Guest agent will not be running."
else
	run_boot_scripts "${LIMA_CIDATA_MNT}/boot.${UNAME}"
fi

# indirect variable lookup, like ${!var} in bash
deref() {
	eval echo \$"$1"
}

if [ -d "${LIMA_CIDATA_MNT}"/provision.data ]; then
	for f in "${LIMA_CIDATA_MNT}"/provision.data/*; do
		filename=$(basename "$f")
		overwrite=$(deref "LIMA_CIDATA_DATAFILE_${filename}_OVERWRITE")
		owner=$(deref "LIMA_CIDATA_DATAFILE_${filename}_OWNER")
		path=$(deref "LIMA_CIDATA_DATAFILE_${filename}_PATH")
		permissions=$(deref "LIMA_CIDATA_DATAFILE_${filename}_PERMISSIONS")
		user="${owner%%:*}"
		if [ -e "$path" ] && [ "$overwrite" = "false" ]; then
			INFO "Not overwriting $path"
		else
			INFO "Copying $f to $path"
			if ! sudo -iu "${user}" mkdir -p "$(dirname "$path")"; then
				WARNING "Failed to create directory for ${path} (as user ${user})"
				WARNING "Falling back to creating directory as root to maintain compatibility"
				mkdir -p "$(dirname "$path")"
			fi
			cp "$f" "$path"
			chown "$owner" "$path"
			chmod "$permissions" "$path"
		fi
	done
fi

if [ -d "${LIMA_CIDATA_MNT}"/provision.yq ]; then
	yq="${LIMA_CIDATA_MNT}/lima-guestagent yq"
	for f in "${LIMA_CIDATA_MNT}"/provision.yq/*; do
		filename=$(basename "${f}")
		format=$(deref "LIMA_CIDATA_YQ_PROVISION_${filename}_FORMAT")
		owner=$(deref "LIMA_CIDATA_YQ_PROVISION_${filename}_OWNER")
		path=$(deref "LIMA_CIDATA_YQ_PROVISION_${filename}_PATH")
		permissions=$(deref "LIMA_CIDATA_YQ_PROVISION_${filename}_PERMISSIONS")
		user="${owner%%:*}"
		# Creating intermediate directories may fail if the user does not have permission.
		# TODO: Create intermediate directories with the specified group ownership.
		if ! sudo -iu "${user}" mkdir -p "$(dirname "${path}")"; then
			WARNING "Failed to create directory for ${path} (as user ${user})"
			CODE=1
			continue
		fi
		# Since CIDATA is mounted with dmode=700,fmode=700,
		# `lima-guestagent yq` cannot be executed by non-root users,
		# and provision.yq/* files cannot be read by non-root users.
		if [ -f "${path}" ]; then
			INFO "Updating ${path}"
			# If the user does not have write permission, it should fail.
			# This avoids changes being made by the wrong user.
			if ! sudo -iu "${user}" test -w "${path}"; then
				WARNING "File ${path} is not writable by user ${user}"
				CODE=1
				continue
			fi
			# Relies on the fact that yq does not change the owner of the existing file.
			if ! ${yq} --inplace --from-file "${f}" --input-format "${format}" --output-format "${format}" "${path}"; then
				WARNING "Failed to update ${path} (as user ${user})"
				CODE=1
				continue
			fi
		else
			if [ "${format}" = "auto" ]; then
				# yq can't determine the output format from non-existing files
				case "${path}" in
				*.csv) format=csv ;;
				*.ini) format=ini ;;
				*.json) format=json ;;
				*.properties) format=properties ;;
				*.toml) format=toml ;;
				*.tsv) format=tsv ;;
				*.xml) format=xml ;;
				*.yaml | *.yml) format=yaml ;;
				*)
					format=yaml
					WARNING "Cannot determine file type for ${path}, using yaml format"
					;;
				esac
			fi
			INFO "Creating ${path}"
			if ! ${yq} --null-input --from-file "${f}" --output-format "${format}" | sudo -iu "${user}" tee "${path}"; then
				WARNING "Failed to create ${path} (as user ${user})"
				CODE=1
				continue
			fi
		fi
		if ! sudo -iu "${user}" chown "${owner}" "${path}"; then
			WARNING "Failed to set owner for ${path} (as user ${user})"
			CODE=1
		fi
		if ! sudo -iu "${user}" chmod "${permissions}" "${path}"; then
			WARNING "Failed to set permissions for ${path} (as user ${user})"
			CODE=1
		fi
	done
fi

if [ -d "${LIMA_CIDATA_MNT}"/provision.system ]; then
	for f in "${LIMA_CIDATA_MNT}"/provision.system/*; do
		INFO "Executing $f"
		if ! "$f"; then
			WARNING "Failed to execute $f"
			CODE=1
		fi
	done
fi

USER_SCRIPT="${LIMA_CIDATA_HOME}/.lima-user-script"
if [ -d "${LIMA_CIDATA_MNT}"/provision.user ]; then
	if [ ! -f /sbin/openrc-run ]; then
		until [ -e "/run/user/${LIMA_CIDATA_UID}/systemd/private" ]; do sleep 3; done
	fi
	params=$(grep -o '^PARAM_[^=]*' "${LIMA_CIDATA_MNT}"/param.env | paste -sd ,)
	for f in "${LIMA_CIDATA_MNT}"/provision.user/*; do
		INFO "Executing $f (as user ${LIMA_CIDATA_USER})"
		cp "$f" "${USER_SCRIPT}"
		chown "${LIMA_CIDATA_USER}" "${USER_SCRIPT}"
		chmod 755 "${USER_SCRIPT}"
		if [ "${UNAME}" != "Linux" ]; then
			WARNING "Provisioning user scripts are not supported on non-Linux platforms"
			CODE=1
		elif ! sudo -iu "${LIMA_CIDATA_USER}" "--preserve-env=${params}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "${USER_SCRIPT}"; then
			WARNING "Failed to execute $f (as user ${LIMA_CIDATA_USER})"
			CODE=1
		fi
		rm "${USER_SCRIPT}"
	done
fi

# Signal that provisioning is done. The instance-id in the meta-data file changes on every boot,
# so any copy from a previous boot cycle will have different content.
cp "${LIMA_CIDATA_MNT}"/meta-data "${RUN}/lima-boot-done"

INFO "Exiting with code $CODE"
exit "$CODE"
