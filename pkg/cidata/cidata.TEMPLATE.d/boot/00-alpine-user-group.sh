#!/bin/sh
# Remove the user embedded in the image,
# and use cloud-init for users and groups.
test -f /etc/alpine-release || exit 0
test "$LIMA_CIDATA_USER" != "alpine" || exit 0

if [ "$(id -u alpine 2>&1)" = "1000" ]; then
	userdel alpine
	rmdir /home/alpine
	cloud-init clean --logs
	reboot
fi
