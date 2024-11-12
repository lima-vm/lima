#!/bin/sh
set -eu

# In vz, the VM lacks an RTC when booting with a kernel image (see: https://developer.apple.com/forums/thread/760344).
# This causes incorrect system time until NTP synchronizes it, leading to TLS errors.
# To avoid TLS errors, this script waits for NTP synchronization if RTC is unavailable.
test ! -c /dev/rtc0 || exit 0

# This script is intended for services running with systemd.
command -v systemctl >/dev/null 2>&1 || exit 0

# Enable `systemd-time-wait-sync.service` to wait for NTP synchronization at an earlier stage.
systemctl enable systemd-time-wait-sync.service

# For the first boot, where the above setting is not yet active, wait for NTP synchronization here.
until ntp_synchronized=$(timedatectl show --property=NTPSynchronized --value) && [ "${ntp_synchronized}" = "yes" ]; do
	time_usec=$(timedatectl show --property=TimeUSec)
	echo "${time_usec}, Waiting for NTP synchronization..."
	sleep 1
done
# Print the result of NTP synchronization
ntp_message=$(timedatectl show-timesync --property=NTPMessage)
time_usec=$(timedatectl show --property=TimeUSec)
echo "${time_usec}, NTP synchronization complete."
echo "${ntp_message}"
