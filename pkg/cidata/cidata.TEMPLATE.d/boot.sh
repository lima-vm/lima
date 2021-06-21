#!/bin/sh
set -eu

INFO(){
  echo "LIMA| $*"
}

WARNING(){
  echo "LIMA| WARNING: $*"
}

# shellcheck disable=SC2163
while read -r line; do export "$line"; done <"${LIMA_CIDATA_MNT}"/lima.env

CODE=0

for f in "${LIMA_CIDATA_MNT}"/boot/*; do
	INFO "Executing $f"
	if ! "$f"; then
		WARNING "Failed to execute $f"
		CODE=1
	fi
done

if [ -d "${LIMA_CIDATA_MNT}"/provision.system ]; then
	for f in "${LIMA_CIDATA_MNT}"/provision.system/*; do
		INFO "Executing $f"
		if ! "$f"; then
			WARNING "Failed to execute $f"
			CODE=1
		fi
	done
fi

if [ -d "${LIMA_CIDATA_MNT}"/provision.user ]; then
	until [ -e "/run/user/${LIMA_CIDATA_UID}}/systemd/private" ]; do sleep 3; done
	for f in "${LIMA_CIDATA_MNT}"/provision.user/*; do
		INFO "Executing $f (as user ${LIMA_CIDATA_USER})"
		if ! sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "$f"; then
			WARNING "Failed to execute $f (as user ${LIMA_CIDATA_USER})"
			CODE=1
		fi
	done
fi

INFO "Exiting with code $CODE"
exit "$CODE"
