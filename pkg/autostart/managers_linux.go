// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

import "github.com/lima-vm/lima/v2/pkg/autostart/systemd"

// DaemonManager is not supported on Linux; use systemd user services instead.
func DaemonManager(_ string) autoStartManager {
	return &notSupportedManager{}
}

// ManagerWith returns the autostart manager for Linux.
// The keepAlive parameter is accepted for API compatibility but ignored;
// systemd restart behavior is configured separately via the unit file.
func ManagerWith(_ bool) autoStartManager {
	return Manager()
}

// Manager returns the autostart manager for Linux.
func Manager() autoStartManager {
	if systemd.IsRunningSystemd() {
		return &TemplateFileBasedManager{
			filePath:              systemd.GetUnitPath,
			template:              systemd.Template,
			enabler:               systemd.EnableDisableUnit,
			autoStartedIdentifier: systemd.AutoStartedUnitName,
			requestStart:          systemd.RequestStart,
			requestStop:           systemd.RequestStop,
		}
	}
	// TODO: add support for non-systemd Linux distros
	return &notSupportedManager{}
}
