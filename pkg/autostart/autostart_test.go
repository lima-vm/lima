package autostart

import (
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestRenderTemplate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping testing on windows host")
	}
	tests := []struct {
		Name          string
		InstanceName  string
		HostOS        string
		Expected      string
		WorkDir       string
		GetExecutable func() (string, error)
	}{
		{
			Name:         "render darwin launchd plist",
			InstanceName: "default",
			HostOS:       "darwin",
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
</plist>`,
			GetExecutable: func() (string, error) {
				return "/limactl", nil
			},
			WorkDir: "/some/path",
		},
		{
			Name:         "render linux systemd service",
			InstanceName: "default",
			HostOS:       "linux",
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
WantedBy=default.target`,
			GetExecutable: func() (string, error) {
				return "/limactl", nil
			},
			WorkDir: "/some/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			tmpl, err := renderTemplate(tt.HostOS, tt.InstanceName, tt.WorkDir, tt.GetExecutable)
			assert.NilError(t, err)
			assert.Equal(t, string(tmpl), tt.Expected)
		})
	}
}

func TestGetFilePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping testing on windows host")
	}
	tests := []struct {
		Name         string
		HostOS       string
		InstanceName string
		HomeEnv      string
		Expected     string
	}{
		{
			Name:         "darwin with docker instance name",
			HostOS:       "darwin",
			InstanceName: "docker",
			Expected:     "Library/LaunchAgents/io.lima-vm.autostart.docker.plist",
		},
		{
			Name:         "linux with docker instance name",
			HostOS:       "linux",
			InstanceName: "docker",
			Expected:     ".config/systemd/user/lima-vm@docker.service",
		},
		{
			Name:         "empty with empty instance name",
			HostOS:       "",
			InstanceName: "",
			Expected:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Check(t, strings.HasSuffix(GetFilePath(tt.HostOS, tt.InstanceName), tt.Expected))
		})
	}
}
