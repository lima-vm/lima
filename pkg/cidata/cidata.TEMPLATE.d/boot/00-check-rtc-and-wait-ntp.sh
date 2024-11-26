#!/bin/sh
set -eu

# In vz, the VM lacks an RTC when booting with a kernel image (see: https://developer.apple.com/forums/thread/760344).
# This causes incorrect system time until NTP synchronizes it, leading to TLS errors.
# To avoid TLS errors, this script waits for NTP synchronization if RTC is unavailable.
test ! -c /dev/rtc0 || exit 0

# This script is intended for services running with systemd.
command -v systemctl >/dev/null 2>&1 || exit 0

echo_with_time_usec() {
	time_usec=$(timedatectl show --property=TimeUSec)
	echo "${time_usec}, ${1}"
}

# For the first boot, where the above setting is not yet active, wait for NTP synchronization here.
max_retry=60 retry=0
until ntp_synchronized=$(timedatectl show --property=NTPSynchronized --value) && [ "${ntp_synchronized}" = "yes" ] ||
	[ "${retry}" -gt "${max_retry}" ]; do
	if [ "${retry}" -eq 0 ]; then
		# If /dev/rtc is not available, the system time set during the Linux kernel build is used.
		# The larger the difference between this system time and the NTP server time, the longer the NTP synchronization will take.
		# By setting the system time to the modification time of this script, which is likely to be closer to the actual time,
		# the NTP synchronization time can be shortened.
		echo_with_time_usec "Setting the system time to the modification time of ${0}."

		# To set the time to a specified time, it is necessary to stop systemd-timesyncd.
		systemctl stop systemd-timesyncd

		# Since `timedatectl set-time` fails if systemd-timesyncd is not stopped,
		# ensure that it is completely stopped before proceeding.
		until pid_of_timesyncd=$(systemctl show systemd-timesyncd --property=MainPID --value) && [ "${pid_of_timesyncd}" -eq 0 ]; do
			echo_with_time_usec "Waiting for systemd-timesyncd to stop..."
			sleep 1
		done

		# Set the system time to the modification time of this script.
		modification_time=$(stat -c %y "${0}")
		echo_with_time_usec "Setting the system time to ${modification_time}."
		timedatectl set-time "${modification_time}"

		# Restart systemd-timesyncd
		systemctl start systemd-timesyncd
	else
		echo_with_time_usec "Waiting for NTP synchronization..."
	fi
	retry=$((retry + 1))
	sleep 1
done
# Print the result of NTP synchronization
ntp_message=$(timedatectl show-timesync --property=NTPMessage)
if [ "${ntp_synchronized}" = "yes" ]; then
	echo_with_time_usec "NTP synchronization complete."
	echo "${ntp_message}"
else
	echo_with_time_usec "NTP synchronization timed out."
	echo "${ntp_message}"
	exit 1
fi
