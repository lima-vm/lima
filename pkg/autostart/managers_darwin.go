// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

import "github.com/lima-vm/lima/v2/pkg/autostart/launchd"

// Manager returns the autostart manager for Darwin.
func Manager() autoStartManager {
	return ManagerWith(true)
}

// ManagerWith returns the autostart manager for Darwin with the given options.
func ManagerWith(keepAlive bool) autoStartManager {
	extra := map[string]string{}
	if keepAlive {
		extra["KeepAlive"] = "true"
	}
	return &TemplateFileBasedManager{
		filePath:              launchd.GetPlistPath,
		template:              launchd.Template,
		enabler:               launchd.EnableDisableService,
		autoStartedIdentifier: launchd.AutoStartedServiceName,
		requestStart:          launchd.RequestStart,
		requestStop:           launchd.RequestStop,
		extraTemplateVars:     extra,
	}
}

// DaemonManager returns an autostart manager for rendering and tracking system LaunchDaemon plists.
// The userName is the macOS user the daemon will run as.
// Note: install/uninstall require privileged operations (writing to /Library/LaunchDaemons/ and
// interacting with the system launchctl domain) that are handled by the `limactl daemon` CLI
// command via sudo rather than by this manager directly.
func DaemonManager(userName string) autoStartManager {
	return &TemplateFileBasedManager{
		filePath:          launchd.GetDaemonPlistPath,
		template:          launchd.DaemonTemplate,
		extraTemplateVars: map[string]string{"UserName": userName},
	}
}
