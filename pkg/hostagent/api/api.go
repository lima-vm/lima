// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

type Info struct {
	// indicate instance is started by launchd or systemd if not empty
	AutoStartedIdentifier string `json:"autoStartedIdentifier,omitempty"`
	// SSHLocalPort is the local port on the host for SSH access to the VM.
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}
