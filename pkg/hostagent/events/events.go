/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

type Event struct {
	Time   time.Time `json:"time,omitempty"`
	Status Status    `json:"status,omitempty"`
}
