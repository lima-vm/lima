// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"time"
)

type Status struct {
	Running bool `json:"running,omitempty"`
	// When Degraded is true, Running must be true as well
	Degraded bool `json:"degraded,omitempty"`
	// When Exiting is true, Running must be false
	Exiting bool `json:"exiting,omitempty"`

	Errors []string `json:"errors,omitempty"`

	SSHLocalPort int `json:"sshLocalPort,omitempty"`

	// Cloud-init progress information
	CloudInitProgress *CloudInitProgress `json:"cloudInitProgress,omitempty"`

	// Port forwarding event
	PortForward *PortForwardEvent `json:"portForward,omitempty"`

	// Vsock forwarder event
	Vsock *VsockEvent `json:"vsock,omitempty"`
}

type CloudInitProgress struct {
	// Current log line from cloud-init
	LogLine string `json:"logLine,omitempty"`
	// Whether cloud-init has completed
	Completed bool `json:"completed,omitempty"`
	// Whether cloud-init monitoring is active
	Active bool `json:"active,omitempty"`
}

type PortForwardEventType string

const (
	PortForwardEventForwarding    PortForwardEventType = "forwarding"
	PortForwardEventNotForwarding PortForwardEventType = "not-forwarding"
	PortForwardEventStopping      PortForwardEventType = "stopping"
	PortForwardEventFailed        PortForwardEventType = "failed"
)

type PortForwardEvent struct {
	Type      PortForwardEventType `json:"type"`
	Protocol  string               `json:"protocol,omitempty"`
	GuestAddr string               `json:"guestAddr,omitempty"`
	HostAddr  string               `json:"hostAddr,omitempty"`
	Error     string               `json:"error,omitempty"`
}

type VsockEventType string

const (
	VsockEventStarted VsockEventType = "started"
	VsockEventSkipped VsockEventType = "skipped"
	VsockEventFailed  VsockEventType = "failed"
)

type VsockEvent struct {
	Type      VsockEventType `json:"type"`
	HostAddr  string         `json:"hostAddr,omitempty"`
	VsockPort int            `json:"vsockPort,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

type Event struct {
	Time   time.Time `json:"time,omitempty"`
	Status Status    `json:"status,omitempty"`
}
