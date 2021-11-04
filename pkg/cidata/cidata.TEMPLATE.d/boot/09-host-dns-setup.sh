#!/bin/sh
set -eux

# Wait until iptables has been installed; 30-install-packages.sh will call this script again
if command -v iptables >/dev/null 2>&1; then
	if [ -n "${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}" ] && [ "${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}" -ne 0 ]; then
		# Only add the rule once
		if ! iptables-save | grep "udp.*${LIMA_CIDATA_SLIRP_GATEWAY}:${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}"; then
			iptables -t nat -A PREROUTING -d "${LIMA_CIDATA_SLIRP_DNS}" -p udp --dport 53 -j DNAT \
				--to-destination "${LIMA_CIDATA_SLIRP_GATEWAY}:${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}"
			iptables -t nat -A OUTPUT -d "${LIMA_CIDATA_SLIRP_DNS}" -p udp --dport 53 -j DNAT \
				--to-destination "${LIMA_CIDATA_SLIRP_GATEWAY}:${LIMA_CIDATA_UDP_DNS_LOCAL_PORT}"
		fi
	fi
	if [ -n "${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}" ] && [ "${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}" -ne 0 ]; then
		# Only add the rule once
		if ! iptables-save | grep "tcp.*${LIMA_CIDATA_SLIRP_GATEWAY}:${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}"; then
			iptables -t nat -A PREROUTING -d "${LIMA_CIDATA_SLIRP_DNS}" -p tcp --dport 53 -j DNAT \
				--to-destination "${LIMA_CIDATA_SLIRP_GATEWAY}:${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}"
			iptables -t nat -A OUTPUT -d "${LIMA_CIDATA_SLIRP_DNS}" -p tcp --dport 53 -j DNAT \
				--to-destination "${LIMA_CIDATA_SLIRP_GATEWAY}:${LIMA_CIDATA_TCP_DNS_LOCAL_PORT}"
		fi
	fi
fi
