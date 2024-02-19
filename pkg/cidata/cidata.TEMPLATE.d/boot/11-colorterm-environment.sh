#!/bin/sh
set -eux

if [ -d /etc/ssh/sshd_config.d ]; then
	if [ -e /etc/ssh/sshd_config.d/10-acceptenv-colorterm.conf ]; then
		exit 0
	fi

	# accept any incoming COLORTERM environment variable
	echo "AcceptEnv COLORTERM" >/etc/ssh/sshd_config.d/10-acceptenv-colorterm.conf
elif [ -e /etc/ssh/sshd_config ]; then
	if grep -q "COLORTERM" /etc/ssh/sshd_config; then
		exit 0
	fi

	# accept any incoming COLORTERM environment variable
	sed -i 's/^AcceptEnv LANG LC_\*$/AcceptEnv COLORTERM LANG LC_*/' /etc/ssh/sshd_config
else
	exit 0
fi

if [ -f /sbin/openrc-run ]; then
	rc-service --ifstarted sshd reload
elif command -v systemctl >/dev/null 2>&1; then
	if systemctl -q is-active ssh; then
		systemctl reload ssh
	elif systemctl -q is-active sshd; then
		systemctl reload sshd
	fi
fi
