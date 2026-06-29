// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

type Info struct {
	// indicate instance is started by launchd or systemd if not empty
	AutoStartedIdentifier string `json:"autoStartedIdentifier,omitempty"`
	// SSHLocalPort is the local port on the host for SSH access to the VM.
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}

// Mount describes a runtime (hot) mount currently active in the instance.
type Mount struct {
	// ID uniquely identifies the hot-mount within the instance (the guest mount point).
	ID string `json:"id"`
	// HostPath is the shared host directory.
	HostPath string `json:"hostPath"`
	// MountPoint is the guest directory the host path is mounted at.
	MountPoint string `json:"mountPoint"`
	// Type is the mount transport: "virtiofs", "9p", or "reverse-sshfs".
	Type string `json:"type"`
	// Writable indicates a read-write mount.
	Writable bool `json:"writable"`
}

// MountRequest is the body of POST /v1/mounts and DELETE /v1/mounts.
type MountRequest struct {
	HostPath   string `json:"hostPath,omitempty"`
	MountPoint string `json:"mountPoint"`
	Type       string `json:"type,omitempty"`
	Writable   bool   `json:"writable,omitempty"`
}
