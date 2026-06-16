// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

import "github.com/lima-vm/lima/v2/pkg/autostart/systemd"

// DaemonManager is not supported on Linux; use systemd user services instead.
func DaemonManager(_ string) autoStartManager {
	return &notSupportedManager{}
}

// Manager returns the autostart manager for Linux, with keep-alive enabled by default.
func Manager() autoStartManager {
	return ManagerWith(true)
}

// ManagerWith returns the autostart manager for Linux. keepAlive controls the systemd
// unit's Restart= directive: on-failure when enabled, no when disabled.
func ManagerWith(keepAlive bool) autoStartManager {
	if !systemd.IsRunningSystemd() {
		// TODO: add support for non-systemd Linux distros
		return &notSupportedManager{}
	}
	restart := "on-failure"
	if !keepAlive {
		restart = "no"
	}
	return &TemplateFileBasedManager{
		filePath:              systemd.GetUnitPath,
		template:              systemd.Template,
		enabler:               systemd.EnableDisableUnit,
		autoStartedIdentifier: systemd.AutoStartedUnitName,
		requestStart:          systemd.RequestStart,
		requestStop:           systemd.RequestStop,
		extraTemplateVars:     map[string]string{"Restart": restart},
	}
}
