//go:build !linux
// +build !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import "github.com/sirupsen/logrus"

type DockerEventMonitor struct{}

func NewDockerEventMonitor(_ []string) (*DockerEventMonitor, error) {
	logrus.Warn("Docker event monitoring is not implemented on this platform")
	return nil, nil
}
