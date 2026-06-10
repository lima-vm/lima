// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

var defaultRemoveDefaults = false

func TestConfig(t *testing.T) {
	args := &TemplateArgs{
		Name:    "default",
		User:    "foo",
		UID:     501,
		Comment: "Foo",
		Home:    "/home/foo.guest",
		Shell:   "/bin/bash",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		MountType: "reverse-sshfs",
	}
	config, err := ExecuteTemplateCloudConfig(args)
	assert.NilError(t, err)
	t.Log(string(config))
	assert.Assert(t, !strings.Contains(string(config), "ca_certs:"))
	assert.Assert(t, !strings.Contains(string(config), "mounts:"))
}

func TestConfigCACerts(t *testing.T) {
	args := &TemplateArgs{
		Name:    "default",
		User:    "foo",
		UID:     501,
		Comment: "Foo",
		Home:    "/home/foo.guest",
		Shell:   "/bin/bash",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		MountType: "reverse-sshfs",
		CACerts: CACerts{
			RemoveDefaults: &defaultRemoveDefaults,
		},
	}
	config, err := ExecuteTemplateCloudConfig(args)
	assert.NilError(t, err)
	t.Log(string(config))
	assert.Assert(t, strings.Contains(string(config), "ca_certs:"))
}

var defaultMounts = []Mount{
	{MountPoint: "/home/foo.guest", Tag: "mount0", Type: "virtiofs", Options: "ro"},
	{MountPoint: "/tmp/lima", Tag: "mount1", Type: "virtiofs"},
}

func TestConfigMounts(t *testing.T) {
	args := &TemplateArgs{
		Name:    "default",
		User:    "foo",
		UID:     501,
		Comment: "Foo",
		Home:    "/home/foo.guest",
		Shell:   "/bin/bash",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		MountType: "virtiofs", // override
		Mounts:    defaultMounts,
	}
	config, err := ExecuteTemplateCloudConfig(args)
	assert.NilError(t, err)
	t.Log(string(config))
	assert.Assert(t, strings.Contains(string(config), "mounts:"))
}

func TestConfigMountsNone(t *testing.T) {
	args := &TemplateArgs{
		Name:    "default",
		User:    "foo",
		UID:     501,
		Comment: "Foo",
		Home:    "/home/foo.guest",
		Shell:   "/bin/bash",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		MountType: "virtiofs", // override
		Mounts:    []Mount{},
	}
	config, err := ExecuteTemplateCloudConfig(args)
	assert.NilError(t, err)
	t.Log(string(config))
	assert.Assert(t, !strings.Contains(string(config), "mounts:"))
}

func TestTemplate(t *testing.T) {
	args := &TemplateArgs{
		Name:  "default",
		User:  "foo",
		UID:   501,
		Home:  "/home/foo.guest",
		Shell: "/bin/bash",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []Mount{
			{MountPoint: "/Users/dummy"},
			{MountPoint: "/Users/dummy/lima"},
		},
		MountType: "reverse-sshfs",
		CACerts: CACerts{
			RemoveDefaults: &defaultRemoveDefaults,
			Trusted:        []Cert{},
		},
	}
	layout, err := ExecuteTemplateCIDataISO(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %#q ===", f.Path)
		b, err := io.ReadAll(f.Reader)
		assert.NilError(t, err)
		t.Log(string(b))
		if f.Path == "user-data" {
			// mounted later
			assert.Assert(t, !strings.Contains(string(b), "mounts:"))
			// ca_certs:
			assert.Assert(t, !strings.Contains(string(b), "trusted:"))
		}
	}
}

func TestTemplate9p(t *testing.T) {
	args := &TemplateArgs{
		Name:  "default",
		User:  "foo",
		UID:   501,
		Home:  "/home/foo.guest",
		Shell: "/bin/bash",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []Mount{
			{Tag: "mount0", MountPoint: "/Users/dummy", Type: "9p", Options: "ro,trans=virtio"},
			{Tag: "mount1", MountPoint: "/Users/dummy/lima", Type: "9p", Options: "rw,trans=virtio"},
		},
		MountType: "9p",
		CACerts: CACerts{
			RemoveDefaults: &defaultRemoveDefaults,
		},
	}
	layout, err := ExecuteTemplateCIDataISO(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %#q ===", f.Path)
		b, err := io.ReadAll(f.Reader)
		assert.NilError(t, err)
		t.Log(string(b))
		if f.Path == "user-data" {
			// mounted at boot
			assert.Assert(t, strings.Contains(string(b), "mounts:"))
		}
	}
}

// TestTemplateNICRename is a regression test for
// https://github.com/lima-vm/lima/issues/4792 (Ubuntu 26.04 first-boot NIC
// rename race, LP: #2136392): user-data must contain the rename/wait bootcmd,
// and network-config must keep set-name and emit "optional: true" only when
// the internal_netplanOptional param is set (it must never reach non-netplan distros,
// where it renders as RequiredForOnline=no and breaks wait-online).
func TestTemplateNICRename(t *testing.T) {
	args := &TemplateArgs{
		Name:         "default",
		User:         "foo",
		UID:          501,
		Home:         "/home/foo.guest",
		Shell:        "/bin/bash",
		SSHPubKeys:   []string{"ssh-rsa dummy foo@example.com"},
		MountType:    "reverse-sshfs",
		OS:           "Linux",
		SlirpNICName: "eth0",
		Networks: []Network{
			{MACAddress: "52:55:55:12:34:56", Interface: "eth0", Metric: 200},
			{MACAddress: "52:55:55:ab:cd:ef", Interface: "lima0", Metric: 300},
		},
	}
	for _, optional := range []bool{false, true} {
		if optional {
			args.Param = map[string]string{"internal_netplanOptional": "true"}
		}
		layout, err := ExecuteTemplateCIDataISO(args)
		assert.NilError(t, err)
		files := make(map[string]string)
		for _, f := range layout {
			b, err := io.ReadAll(f.Reader)
			assert.NilError(t, err)
			files[f.Path] = string(b)
		}
		assert.Assert(t, strings.Contains(files["user-data"], "52:55:55:12:34:56=eth0"))
		assert.Assert(t, strings.Contains(files["user-data"], "52:55:55:ab:cd:ef=lima0"))
		assert.Assert(t, strings.Contains(files["network-config"], "set-name: eth0"))
		assert.Equal(t, strings.Contains(files["network-config"], "optional: true"), optional)
	}
}

func TestExecuteTemplateWindowsISO(t *testing.T) {
	args := &TemplateArgs{
		Name:                   "windows",
		User:                   "windows-user",
		WindowsInitialPassword: "dummy-password",
	}

	layout, err := ExecuteTemplateWindowsISO(args)
	assert.NilError(t, err)
	for _, f := range layout {
		b, err := io.ReadAll(f.Reader)
		assert.NilError(t, err)
		switch f.Path {
		case "autounattend.xml":
			t.Log(string(b))
			assert.Assert(t, strings.Contains(
				string(b),
				`<Username>windows-user</Username>`,
			))
			assert.Assert(t, strings.Contains(
				string(b),
				`<Value>dummy-password</Value>`,
			))
			// It confirms the file has UEFI setting.
			assert.Assert(t, strings.Contains(
				string(b),
				`<Type>EFI</Type>`,
			))
			// It confirms the file doesn't have BIOS setting.
			assert.Assert(t, !strings.Contains(
				string(b),
				`<Label>BIOS</Label>`,
			))
		case "first_logon.ps1":
			assert.Assert(t, strings.Contains(
				string(b),
				`$logfile = "C:\Users\windows-user\lima-setup.log"`,
			))
		}
	}
}
