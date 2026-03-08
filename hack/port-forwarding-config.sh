# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Shared port forwarding configuration for Lima tests.
# Sourced by both hack/test-templates.sh and hack/bats/tests/port-forwarding.bats.

# shellcheck shell=bash

# Detect the host's external IPv4 address.
get_host_ipv4() {
	local ipv4=""
	if [[ $(uname -s) == "Darwin" ]]; then
		ipv4=$(system_profiler SPNetworkDataType -json 2>/dev/null |
			jq -r 'first(.SPNetworkDataType[] | select(.ip_address) | .ip_address) | first')
	elif [[ $(uname -o 2>/dev/null) == "Msys" ]]; then
		# hostname -I doesn't exist on MSYS2; use .NET DNS resolver (same as old Perl gethostbyname)
		# shellcheck disable=SC2016 # $_ is PowerShell syntax, not bash
		ipv4=$(powershell.exe -NoProfile -Command \
			'[System.Net.Dns]::GetHostAddresses((hostname)) | Where-Object {$_.AddressFamily -eq "InterNetwork" -and $_.IPAddressToString -ne "127.0.0.1"} | Select-Object -First 1 -ExpandProperty IPAddressToString' \
			2>/dev/null | tr -d '\r')
	else
		# Linux: first non-loopback IPv4 from hostname -I
		ipv4=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -v ':' | grep -v '^127\.' | head -1)
	fi
	# Fallback
	if [[ -z $ipv4 || $ipv4 == "null" ]]; then
		ipv4="127.0.0.1"
	fi
	echo "$ipv4"
}

# Port forwarding rules (YAML) with interleaved test specs (comments).
# Placeholders "HOST_IPV4" and "SOCK_DIR/" are replaced at runtime.
#
# Test spec comment format:
#   # forward: <guest_ip> <guest_port> -> <host_ip> <host_port>
#   # forward: <guest_ip> <guest_port> -> <socket_path>
#   # ignore:  <guest_ip> <guest_port>
#   # skip:    <description>
port_forwards_block() {
	cat <<'RULES'
portForwards:
# skip: 127.0.0.1 22 -> 127.0.0.1 2222
# skip: 127.0.0.1 SSH_LOCAL_PORT

- guestIP: 127.0.0.2
  guestPortRange: [3000, 3009]
  hostPortRange: [2000, 2009]
  ignore: true

- guestIP: 0.0.0.0
  guestIPMustBeZero: false
  guestPortRange: [3010, 3019]
  hostPortRange: [2010, 2019]
  ignore: true

- guestIP: 0.0.0.0
  guestIPMustBeZero: false
  guestPortRange: [3000, 3029]
  hostPortRange: [2000, 2029]

# The following rule is completely shadowed by the previous one and has no effect
- guestIP: 0.0.0.0
  guestIPMustBeZero: false
  guestPortRange: [3020, 3029]
  hostPortRange: [2020, 2029]
  ignore: true

  # ignore:  127.0.0.2 3000
  # forward: 127.0.0.3 3001 -> 127.0.0.1 2001

  # Blocking 127.0.0.2 cannot block forwarding from 0.0.0.0
  # forward: 0.0.0.0   3002 -> 127.0.0.1 2002

  # Blocking 0.0.0.0 will block forwarding from any interface because guestIPMustBeZero is false
  # ignore: 0.0.0.0   3010
  # ignore: 127.0.0.1 3011

  # Forwarding from 0.0.0.0 works for any interface (including IPv6)
  # The "ignore" rule above has no effect because the previous rule already matched.
  # forward: 127.0.0.2 3020 -> 127.0.0.1 2020
  # forward: 127.0.0.1 3021 -> 127.0.0.1 2021
  # forward: 0.0.0.0   3022 -> 127.0.0.1 2022
  # forward: ::        3023 -> 127.0.0.1 2023
  # forward: ::1       3024 -> 127.0.0.1 2024

- guestPortRange: [3030, 3039]
  hostPortRange: [2030, 2039]
  hostIP: HOST_IPV4

  # forward: 127.0.0.1 3030 -> HOST_IPV4 2030
  # forward: 0.0.0.0   3031 -> HOST_IPV4 2031
  # forward: ::        3032 -> HOST_IPV4 2032
  # forward: ::1       3033 -> HOST_IPV4 2033

- guestPortRange: [300, 304]

  # forward: 127.0.0.1    300 -> 127.0.0.1 300
  # forward: 0.0.0.0      301 -> 127.0.0.1 301
  # forward: ::           302 -> 127.0.0.1 302
  # forward: ::1          303 -> 127.0.0.1 303
  # ignore:  192.168.5.15 304 -> 127.0.0.1 304

- guestPortRange: [305, 309]
  guestIPMustBeZero: false

  # forward: 127.0.0.1    325 -> 127.0.0.1 325
  # forward: 0.0.0.0      326 -> 127.0.0.1 326
  # forward: ::           327 -> 127.0.0.1 327
  # forward: ::1          328 -> 127.0.0.1 328
  # ignore:  192.168.5.15 329 -> 127.0.0.1 329

- guestPortRange: [310, 314]
  hostIP: 0.0.0.0

  # forward: 127.0.0.1    310 -> 0.0.0.0 310
  # forward: 0.0.0.0      311 -> 0.0.0.0 311
  # forward: ::           312 -> 0.0.0.0 312
  # forward: ::1          313 -> 0.0.0.0 313
  # ignore:  192.168.5.15 314 -> 0.0.0.0 314

- guestPortRange: [315, 319]
  guestIPMustBeZero: false
  hostIP: 0.0.0.0

  # forward: 127.0.0.1    315 -> 0.0.0.0 315
  # forward: 0.0.0.0      316 -> 0.0.0.0 316
  # forward: ::           317 -> 0.0.0.0 317
  # forward: ::1          318 -> 0.0.0.0 318
  # ignore:  192.168.5.15 319 -> 0.0.0.0 319

  # Things we can't test:
  # - Accessing a forward from a different interface (e.g. connect to HOST_IPV4 to connect to 0.0.0.0)
  # - failed forward to privileged port

- guestIP: "192.168.5.15"
  guestPortRange: [4000, 4009]
  hostIP: "HOST_IPV4"

  # forward: 192.168.5.15 4000 -> HOST_IPV4 4000

- guestIP: "::1"
  guestPortRange: [4010, 4019]
  hostIP: "::"

  # forward: ::1 4010 -> :: 4010

- guestIP: "::"
  guestPortRange: [4020, 4029]
  hostIP: "HOST_IPV4"

  # forward: 127.0.0.1    4020 -> HOST_IPV4 4020
  # forward: 127.0.0.2    4021 -> HOST_IPV4 4021
  # forward: 192.168.5.15 4022 -> HOST_IPV4 4022
  # forward: 0.0.0.0      4023 -> HOST_IPV4 4023
  # forward: ::           4024 -> HOST_IPV4 4024
  # forward: ::1          4025 -> HOST_IPV4 4025

- guestIP: "0.0.0.0"
  guestIPMustBeZero: false
  guestPortRange: [4030, 4039]
  hostIP: "HOST_IPV4"

  # forward: 127.0.0.1    4030 -> HOST_IPV4 4030
  # forward: 127.0.0.2    4031 -> HOST_IPV4 4031
  # forward: 192.168.5.15 4032 -> HOST_IPV4 4032
  # forward: 0.0.0.0      4033 -> HOST_IPV4 4033
  # forward: ::           4034 -> HOST_IPV4 4034
  # forward: ::1          4035 -> HOST_IPV4 4035

- guestIPMustBeZero: true
  guestPortRange: [4040, 4049]

- guestIP: "0.0.0.0"
  guestIPMustBeZero: false
  guestPortRange: [4040, 4049]
  ignore: true

  # forward: 0.0.0.0        4040 -> 127.0.0.1 4040
  # forward: ::             4041 -> 127.0.0.1 4041
  # ignore:  127.0.0.1      4043 -> 127.0.0.1 4043
  # ignore:  192.168.5.15   4044 -> 127.0.0.1 4044

# This rule exists to test `nerdctl run` binding to 0.0.0.0 by default,
# and making sure it gets forwarded to the external host IP.
# The actual test code is in test-example.sh in the "port-forwarding" block.
- guestIPMustBeZero: true
  guestPort: 8888
  hostIP: 0.0.0.0

- guestPort: 5000
  hostSocket: port5000.sock

  # forward: 127.0.0.1    5000 -> SOCK_DIR/port5000.sock

- guestPort: 5001
  hostSocket: port5001.sock

  # ignore:  192.168.5.15 5001 -> SOCK_DIR/port5001.sock

- guestPort: 5002
  guestIPMustBeZero: false
  hostSocket: port5002.sock

  # forward: 127.0.0.1    5002 -> SOCK_DIR/port5002.sock

- guestPort: 5003
  guestIPMustBeZero: false
  hostSocket: port5003.sock

  # ignore:  192.168.5.15 5003 -> SOCK_DIR/port5003.sock
RULES
}

# Generate port forwards YAML with HOST_IPV4 placeholder replaced.
generate_port_forwards_yaml() {
	local host_ipv4=$1
	port_forwards_block | sed "s/HOST_IPV4/${host_ipv4}/g"
}
