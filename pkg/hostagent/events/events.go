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

	// Requirement progress update — one event per state transition, see
	// RequirementProgress for fields.
	RequirementProgress *RequirementProgress `json:"requirementProgress,omitempty"`
}

// RequirementProgress is emitted by the hostagent on every state transition
// of a startup-requirement check, so the limactl-side watcher can render
// progress with a TTY-aware in-place flip (🕐 -> ✅) when stdout is a
// terminal, and fall back to two log lines otherwise.
type RequirementProgress struct {
	// Step is the 1-based index of this requirement across the unified
	// essential / optional / guest-agent / final groups.
	Step int `json:"step"`
	// Total is the total number of steps across all groups for this boot.
	Total int `json:"total"`
	// Description is the human-readable name of the requirement, already
	// capitalized for display.
	Description string `json:"description"`
	// Suffix is an optional trailing annotation shown only while the step
	// is pending (e.g. " (essential)").
	Suffix string `json:"suffix,omitempty"`
	// Done is true when the requirement has been satisfied. The first
	// event for each step is emitted with Done=false; the second with
	// Done=true.
	Done bool `json:"done,omitempty"`
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
