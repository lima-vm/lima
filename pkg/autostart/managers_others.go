//go:build !darwin && !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

// Manager returns a notSupportedManager for unsupported OSes.
func Manager() autoStartManager {
	return &notSupportedManager{}
}

// DaemonManager is not supported on this OS.
func DaemonManager(_ string) autoStartManager {
	return &notSupportedManager{}
}

// ManagerWith is not supported on this OS.
func ManagerWith(_ bool) autoStartManager {
	return &notSupportedManager{}
}
