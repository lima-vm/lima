// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

const (
	SlirpNICName = "eth0"
	// CIDR is intentionally hardcoded to 192.168.5.0/24, as each of QEMU has its own independent slirp network.
	SlirpNetwork   = "192.168.5.0/24"
	SlirpGateway   = "192.168.5.2"
	SlirpIPAddress = "192.168.5.15"

	SocketVMNet       = "socket_vmnet"
	LimaPrivilegedNet = "lima-privileged-net"

	// maxIfNameLen is the longest name the kernel accepts for a network
	// interface (IFNAMSIZ-1). Both the bridge and the tap names below have to fit
	// into it; Validate() rejects network names that would overflow the bridge.
	maxIfNameLen = 15
	// bridgePrefix is prepended to the network name to name the bridge that
	// lima-privileged-net creates for "shared" and "host" networks.
	bridgePrefix = "lima-"
	// tapPrefix is prepended to a hash of the instance and network name.
	tapPrefix = "limatap"
	// tapDigits is the number of hex digits following tapPrefix.
	// len(tapPrefix)+tapDigits must not exceed maxIfNameLen.
	tapDigits = 8

	// privilegedSubdir is the libexec/lima subdirectory reserved for the helpers
	// that run with elevated privileges.
	privilegedSubdir = "privileged"
)
