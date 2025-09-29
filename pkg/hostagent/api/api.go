// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

type Info struct {
	SSHLocalPort int    `json:"sshLocalPort,omitempty"`
	GuestIPv4Address      string `json:"guestIPv4Address,omitempty"`
}
