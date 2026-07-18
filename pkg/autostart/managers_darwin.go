// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

import "github.com/lima-vm/lima/v2/pkg/autostart/launchd"

// Manager returns the autostart manager for Darwin.
func Manager() autoStartManager {
	return &TemplateFileBasedManager{
		filePath:              launchd.GetPlistPath,
		template:              launchd.Template,
		enabler:               launchd.EnableDisableService,
		autoStartedIdentifier: launchd.AutoStartedServiceName,
		requestStart:          launchd.RequestStart,
		requestStop:           launchd.RequestStop,
	}
}
