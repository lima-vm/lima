// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

type Info struct {
	// Guest IP address directly accessible from the host.
	GuestIPAddress string `json:"guestIPAddress,omitempty"`
	// SSH local port on the host forwarded to the guest's port 22.
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}
