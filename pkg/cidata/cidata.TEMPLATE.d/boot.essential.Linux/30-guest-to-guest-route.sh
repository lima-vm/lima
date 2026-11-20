#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# macOS keeps vzNAT guests from reaching each other directly, but the host
# forwards IP traffic between them. So for each network that sets
# guestToGuest, this script adds a route for the network's subnet via its
# gateway. It reads both from the interface's DHCP lease, because macOS picks
# the subnet. The guest's network service owns the route, so the route comes
# back whenever the service reconfigures the interface.

set -eu

HOOK=/usr/lib/dhcpcd/dhcpcd-hooks/90-lima-guest-to-guest
DROPIN=90-lima-guest-to-guest.conf

# network_file prints the path of the network file that systemd-networkd uses
# for interface $1, and fails if networkd doesn't report one.
network_file() {
	FILE=$(networkctl status "$1" | sed -n 's/^ *Network File: //p')
	[ -n "${FILE}" ] && [ "${FILE}" != n/a ] && echo "${FILE}"
}

# An earlier boot may have written a networkd drop-in for an interface whose
# network no longer sets guestToGuest. So keep the drop-ins of the network
# files that the listed interfaces use, and remove every other one.
# NetworkManager doesn't need this, because cloud-init rewrites its
# connections on every boot.
KEEP=
if [ -n "${LIMA_CIDATA_GUEST_TO_GUEST_ROUTES}" ] && systemctl is-active -q systemd-networkd 2>/dev/null; then
	for ROUTE in ${LIMA_CIDATA_GUEST_TO_GUEST_ROUTES}; do
		if ! FILE=$(network_file "${ROUTE%,*}" 2>/dev/null); then
			# networkd doesn't report this interface's network file, so this
			# script can't tell which drop-in to keep. Keep every drop-in on
			# this boot.
			KEEP=all
			break
		fi
		KEEP="${KEEP} /etc/systemd/network/${FILE##*/}.d/${DROPIN}"
	done
fi
RELOAD=
for f in /etc/systemd/network/*.network.d/"${DROPIN}"; do
	[ "${KEEP}" != all ] || break
	case "${KEEP} " in
	*" ${f} "*) ;;
	*)
		if [ -e "${f}" ]; then
			rm -f "${f}"
			RELOAD=1
		fi
		;;
	esac
done
# networkd reads the drop-ins when it starts, so it needs a reload only while
# it runs.
if [ -n "${RELOAD}" ] && systemctl is-active -q systemd-networkd 2>/dev/null; then
	networkctl reload
fi

if [ -z "${LIMA_CIDATA_GUEST_TO_GUEST_ROUTES}" ]; then
	rm -f "${HOOK}"
	exit 0
fi

# Alpine's dhcpcd passes every lease to its hooks, so a hook adds the route.
# dhcpcd-run-hooks sources the hook, so the hook must not exit.
if [ -e /etc/alpine-release ] && [ -d "${HOOK%/*}" ]; then
	{
		echo "# Lima rewrites this file on every boot, from LIMA_CIDATA_GUEST_TO_GUEST_ROUTES."
		for ROUTE in ${LIMA_CIDATA_GUEST_TO_GUEST_ROUTES}; do
			INTERFACE=${ROUTE%,*}
			METRIC=${ROUTE#*,}
			cat <<EOF
if [ "\$interface" = ${INTERFACE} ] && [ -n "\${new_network_number:-}" ] && [ -n "\${new_routers:-}" ]; then
	case "\$reason" in
	BOUND | REBOOT | RENEW | REBIND | STATIC) ip route replace "\$new_network_number/\$new_subnet_cidr" via "\${new_routers%% *}" dev ${INTERFACE} metric ${METRIC} ;;
	esac
fi
EOF
		done
	} >"${HOOK}"
	# dhcpcd may have bound the lease before the hook existed. Rebinding runs
	# the hooks again.
	for ROUTE in ${LIMA_CIDATA_GUEST_TO_GUEST_ROUTES}; do
		INTERFACE=${ROUTE%,*}
		if ip -4 addr show dev "${INTERFACE}" | grep -q "inet "; then
			if ! OUT=$(dhcpcd -n "${INTERFACE}" 2>&1); then
				echo >&2 "WARNING: dhcpcd -n ${INTERFACE} failed, so the guestToGuest route is missing until dhcpcd renews the lease: ${OUT}"
			fi
		fi
	done
	exit 0
fi

for ROUTE in ${LIMA_CIDATA_GUEST_TO_GUEST_ROUTES}; do
	INTERFACE=${ROUTE%,*}
	METRIC=${ROUTE#*,}

	# Don't wait for the lease when the guest runs a network service that the
	# code below can't configure, such as wicked.
	if ! systemctl is-active -q systemd-networkd 2>/dev/null && ! systemctl is-active -q NetworkManager 2>/dev/null; then
		echo >&2 "WARNING: neither systemd-networkd nor NetworkManager is running; not adding the guestToGuest route for ${INTERFACE}"
		continue
	fi

	# This script can start before the interface has its lease.
	CIDR=
	GATEWAY=
	i=0
	while [ "${i}" -lt 600 ]; do
		CIDR=$(ip -4 -o addr show dev "${INTERFACE}" scope global | awk '{print $4; exit}')
		GATEWAY=$(ip -4 route show default dev "${INTERFACE}" | awk '{print $3; exit}')
		if [ -n "${CIDR}" ] && [ -n "${GATEWAY}" ]; then
			break
		fi
		i=$((i + 1))
		sleep 0.1
	done
	if [ -z "${CIDR}" ] || [ -z "${GATEWAY}" ]; then
		echo >&2 "WARNING: ${INTERFACE} did not get both an IPv4 address and a default route within 60 seconds; not adding the guestToGuest route"
		continue
	fi

	# The subnet is the lease's address with the host bits cleared.
	ADDR=${CIDR%/*}
	LEN=${CIDR#*/}
	IFS=. read -r A B C D <<EOF
${ADDR}
EOF
	NET=$((((A << 24) | (B << 16) | (C << 8) | D) & (0xffffffff << (32 - LEN))))
	SUBNET="$((NET >> 24)).$(((NET >> 16) & 255)).$(((NET >> 8) & 255)).$((NET & 255))/${LEN}"

	if systemctl is-active -q systemd-networkd && FILE=$(network_file "${INTERFACE}"); then
		# systemd 256 and later skip a Gateway=_dhcp4 route inside the lease's
		# own prefix, so the drop-in names the gateway, as netplan does for a
		# static route. A reload reconfigures the interface, so it runs only
		# when the drop-in changes.
		DIR=/etc/systemd/network/${FILE##*/}.d
		CONF=$(printf '[Route]\nDestination=%s\nGateway=%s\nGatewayOnLink=yes\nMetric=%s\n' "${SUBNET}" "${GATEWAY}" "${METRIC}")
		CURRENT=$(cat "${DIR}/${DROPIN}" 2>/dev/null || true)
		if [ "${CURRENT}" != "${CONF}" ]; then
			mkdir -p "${DIR}"
			printf '%s\n' "${CONF}" >"${DIR}/${DROPIN}"
			networkctl reload
		fi
	elif systemctl is-active -q NetworkManager && CONN=$(nmcli -g GENERAL.CONNECTION device show "${INTERFACE}") && [ -n "${CONN}" ]; then
		# cloud-init rewrites the connection on every boot, which drops the route.
		nmcli connection modify "${CONN}" ipv4.routes "${SUBNET} ${GATEWAY} ${METRIC}"
		nmcli device reapply "${INTERFACE}"
	else
		echo >&2 "WARNING: neither systemd-networkd nor NetworkManager manages ${INTERFACE}; not adding the guestToGuest route"
	fi
done
