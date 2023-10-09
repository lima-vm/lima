#!/bin/sh
set -eux

# This script prepares Alpine for lima; there is nothing in here for other distros
test -f /etc/alpine-release || exit 0

# need to update /etc/resolv.conf
rc-service networking restart
