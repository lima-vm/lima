//go:build !linux
// +build !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import "github.com/sirupsen/logrus"

type ContainerdEventMonitor struct{}

func NewContainerdEventMonitor(_ []string) (*ContainerdEventMonitor, error) {
	logrus.Warn("Containerd event monitoring is not implemented on this platform")
	return nil, nil
}
