// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

import "github.com/lima-vm/lima/v2/pkg/autostart/systemd"

// DaemonManager is not supported on Linux; use systemd user services instead.
func DaemonManager(_ string) autoStartManager {
	return &notSupportedManager{}
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
