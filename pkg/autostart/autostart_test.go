// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package autostart

import (
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/autostart/launchd"
	"github.com/lima-vm/lima/v2/pkg/autostart/systemd"
)

var (
	Launchd = &TemplateFileBasedManager{
		filePath:              launchd.GetPlistPath,
		template:              launchd.Template,
		enabler:               launchd.EnableDisableService,
		autoStartedIdentifier: launchd.AutoStartedServiceName,
		requestStart:          launchd.RequestStart,
		requestStop:           launchd.RequestStop,
	}
	Systemd = &TemplateFileBasedManager{
		filePath:              systemd.GetUnitPath,
		template:              systemd.Template,
		enabler:               systemd.EnableDisableUnit,
		autoStartedIdentifier: systemd.AutoStartedUnitName,
		requestStart:          systemd.RequestStart,
		requestStop:           systemd.RequestStop,
	}
)

func TestRenderTemplate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping testing on windows host")
	}
	tests := []struct {
		Manager       *TemplateFileBasedManager
		Name          string
		InstanceName  string
		Expected      string
		WorkDir       string
		GetExecutable func() (string, error)
	}{
		{
			Manager:      Launchd,
			Name:         "render darwin launchd plist",
			InstanceName: "default",
			Expected: `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>io.lima-vm.autostart.default</string>
	<key>ProgramArguments</key>
	<array>
		<string>/limactl</string>
		<string>start</string>
		<string>default</string>
		<string>--foreground</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardErrorPath</key>
	<string>launchd.stderr.log</string>
	<key>StandardOutPath</key>
	<string>launchd.stdout.log</string>
	<key>WorkingDirectory</key>
	<string>/some/path</string>
	<key>ProcessType</key>
	<string>Background</string>
</dict>
</plist>
`,
			GetExecutable: func() (string, error) {
				return "/limactl", nil
			},
			WorkDir: "/some/path",
		},
		{
			Manager:      Systemd,
			Name:         "render linux systemd service",
			InstanceName: "default",
			Expected: `[Unit]
Description=Lima - Linux virtual machines, with a focus on running containers.
Documentation=man:lima(1)

[Service]
ExecStart=/limactl start %i --foreground
WorkingDirectory=%h
Type=simple
TimeoutSec=10
Restart=on-failure

[Install]
WantedBy=default.target
`,
			GetExecutable: func() (string, error) {
				return "/limactl", nil
			},
			WorkDir: "/some/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			tmpl, err := tt.Manager.renderTemplate(tt.InstanceName, tt.WorkDir, tt.GetExecutable)
			assert.NilError(t, err)
			assert.Equal(t, string(tmpl), tt.Expected)
		})
	}
}
