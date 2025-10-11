// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"net"
	"time"
)

type Status struct {
	Running bool `json:"running,omitempty"`
	// When Degraded is true, Running must be true as well
	Degraded bool `json:"degraded,omitempty"`
	// When Exiting is true, Running must be false
	Exiting bool `json:"exiting,omitempty"`

	Errors []string `json:"errors,omitempty"`

	// Guest IP address directly accessible from the host.
	GuestIP net.IP `json:"guestIP,omitempty"`
	// SSH local port on the host forwarded to the guest's port 22.
	SSHLocalPort int `json:"sshLocalPort,omitempty"`

	// Cloud-init progress information
	CloudInitProgress *CloudInitProgress `json:"cloudInitProgress,omitempty"`
}

type CloudInitProgress struct {
	// Current log line from cloud-init
	LogLine string `json:"logLine,omitempty"`
	// Whether cloud-init has completed
	Completed bool `json:"completed,omitempty"`
	// Whether cloud-init monitoring is active
	Active bool `json:"active,omitempty"`
}

type Event struct {
	Time   time.Time `json:"time,omitempty"`
	Status Status    `json:"status,omitempty"`
}
