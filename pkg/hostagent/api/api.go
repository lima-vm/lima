// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

import "net"

type Info struct {
	// Guest IP address directly accessible from the host.
	GuestIP net.IP `json:"guestIP,omitempty"`
	// SSH local port on the host forwarded to the guest's port 22.
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}
