// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cloudinittypes

type MetaData struct {
	InstanceID    string `yaml:"instance-id,omitempty"`
	LocalHostname string `yaml:"local-hostname,omitempty"`
}
