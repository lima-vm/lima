//go:build !linux
// +build !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package events

import "github.com/sirupsen/logrus"

type KubeServiceWatcher struct{}

func NewKubeServiceWatcher(_ []string) *KubeServiceWatcher {
	logrus.Warn("NewKubeServiceWatcher is not implemented on this platform")
	return nil
}
