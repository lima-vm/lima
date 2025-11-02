//go:build !darwin && !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

// Manager returns a notSupportedManager for unsupported OSes.
func Manager() autoStartManager {
	return &notSupportedManager{}
}
