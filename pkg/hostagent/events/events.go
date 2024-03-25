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
}

type Requirement struct {
	Label       string `json:"label"`
	Number      int    `json:"number"`
	Count       int    `json:"count"`
	Description string `json:"description"`
	Index       int    `json:"index"`
	Total       int    `json:"total"`
}

type Event struct {
	Time        time.Time   `json:"time,omitempty"`
	Status      Status      `json:"status,omitempty"`
	Requirement Requirement `json:"requirement,omitempty"`
}
