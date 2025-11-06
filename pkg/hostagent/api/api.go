// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

import "net"

type Info struct {
	// indicate instance is started by launchd or systemd if not empty
	AutoStartedIdentifier string `json:"autoStartedIdentifier,omitempty"`
	// Guest IP address directly accessible from the host.
	GuestIP net.IP `json:"guestIP,omitempty"`
	// SSH local port on the host forwarded to the guest's port 22.
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}
