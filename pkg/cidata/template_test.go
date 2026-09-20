// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
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

// TestTemplateDHCPUseDomains is a regression test for
// https://github.com/lima-vm/lima/issues/5515: Ubuntu 24.04 guests inherit the
// DHCP search domain "local", which makes hostname qualification and sudo take
// several seconds. dhcp4-overrides.use-domains must be false on every interface
// so additional NICs cannot independently supply the domain.
func TestTemplateDHCPUseDomains(t *testing.T) {
	args := &TemplateArgs{
		Name:         "default",
		User:         "foo",
		UID:          501,
		Home:         "/home/foo.guest",
		Shell:        "/bin/bash",
		SSHPubKeys:   []string{"ssh-rsa dummy foo@example.com"},
		MountType:    "reverse-sshfs",
		SlirpNICName: "eth0",
		Networks: []Network{
			{MACAddress: "52:55:55:12:34:56", Interface: "eth0", Metric: 200},
			{MACAddress: "52:55:55:ab:cd:ef", Interface: "lima0", Metric: 300},
		},
	}

	type dhcp4Overrides struct {
		RouteMetric uint32 `yaml:"route-metric"`
		UseDomains  *bool  `yaml:"use-domains"`
	}
	type nameservers struct {
		Addresses []string `yaml:"addresses"`
	}
	type ethernet struct {
		Match struct {
			MACAddress string `yaml:"macaddress"`
		} `yaml:"match"`
		DHCP4          bool           `yaml:"dhcp4"`
		SetName        string         `yaml:"set-name"`
		DHCP4Overrides dhcp4Overrides `yaml:"dhcp4-overrides"`
		Nameservers    *nameservers   `yaml:"nameservers"`
	}
	type networkConfig struct {
		Version   int                 `yaml:"version"`
		Ethernets map[string]ethernet `yaml:"ethernets"`
	}

	for _, tc := range []struct {
		name         string
		dnsAddresses []string
	}{
		{name: "empty DNSAddresses"},
		{name: "populated DNSAddresses", dnsAddresses: []string{"192.0.2.53", "192.0.2.54"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args.DNSAddresses = tc.dnsAddresses
			layout, err := ExecuteTemplateCIDataISO(args)
			assert.NilError(t, err)

			var raw []byte
			for _, f := range layout {
				if f.Path != "network-config" {
					continue
				}
				raw, err = io.ReadAll(f.Reader)
				assert.NilError(t, err)
			}
			assert.Assert(t, len(raw) > 0, "network-config missing from ISO layout")

			var cfg networkConfig
			assert.NilError(t, yaml.Unmarshal(raw, &cfg))
			assert.Equal(t, len(cfg.Ethernets), 2)

			eth0, ok := cfg.Ethernets["eth0"]
			assert.Assert(t, ok, "eth0 missing")
			lima0, ok := cfg.Ethernets["lima0"]
			assert.Assert(t, ok, "lima0 missing")

			for name, iface := range map[string]ethernet{"eth0": eth0, "lima0": lima0} {
				assert.Equal(t, iface.DHCP4, true, name)
				assert.Assert(t, iface.DHCP4Overrides.UseDomains != nil, "%s: use-domains must be set", name)
				assert.Equal(t, *iface.DHCP4Overrides.UseDomains, false, name)
				assert.Equal(t, iface.SetName, name)
			}

			assert.Equal(t, eth0.Match.MACAddress, "52:55:55:12:34:56")
			assert.Equal(t, lima0.Match.MACAddress, "52:55:55:ab:cd:ef")
			assert.Equal(t, eth0.DHCP4Overrides.RouteMetric, uint32(200))
			assert.Equal(t, lima0.DHCP4Overrides.RouteMetric, uint32(300))

			if len(tc.dnsAddresses) > 0 {
				assert.Assert(t, eth0.Nameservers != nil, "explicit nameservers must remain on the primary interface")
				assert.DeepEqual(t, eth0.Nameservers.Addresses, tc.dnsAddresses)
				assert.Assert(t, lima0.Nameservers == nil, "nameservers must not appear on additional interfaces")
			} else {
				assert.Assert(t, eth0.Nameservers == nil, "empty DNSAddresses must not introduce a nameservers block")
				assert.Assert(t, lima0.Nameservers == nil)
			}
		})
	}
}

func TestExecuteTemplateWindowsISO(t *testing.T) {
	testCases := []struct {
		name                        string
		args                        *TemplateArgs
		expectedAutounattendStrings []string
		expectedFirstLogonStrings   []string
	}{
		{
			name: "windows server 2025",
			args: &TemplateArgs{
				Name:                   "windows",
				UID:                    501,
				Home:                   "/home/foo.guest",
				Shell:                  "powershell.exe",
				SSHPubKeys:             []string{"ssh-rsa dummy foo@example.com"},
				Arch:                   limatype.X8664,
				User:                   "windows-user",
				WindowsInitialPassword: "dummy-password",
				TPM:                    true,
				IsWindowsServer:        true,
			},
			expectedAutounattendStrings: []string{
				`<Path>E:\viostor\2k25\amd64</Path>`,
				`<Username>windows-user</Username>`,
				`<Value>dummy-password</Value>`,
				`<Type>EFI</Type>`,
				`<Value>1</Value>`,
			},
			expectedFirstLogonStrings: []string{
				`$logfile = "C:\Users\windows-user\lima-setup.log"`,
			},
		},
		{
			name: "windows 11 x86_64",
			args: &TemplateArgs{
				Name:                   "windows",
				UID:                    501,
				Home:                   "/home/foo.guest",
				Shell:                  "powershell.exe",
				SSHPubKeys:             []string{"ssh-rsa dummy foo@example.com"},
				Arch:                   limatype.X8664,
				User:                   "windows-user",
				WindowsInitialPassword: "dummy-password",
				TPM:                    true,
			},
			expectedAutounattendStrings: []string{
				`<Path>E:\viostor\w11\amd64</Path>`,
				`<Username>windows-user</Username>`,
				`<Value>dummy-password</Value>`,
				`<Type>EFI</Type>`,
				`<Value>6</Value>`,
			},
			expectedFirstLogonStrings: []string{
				`$logfile = "C:\Users\windows-user\lima-setup.log"`,
			},
		},
		{
			name: "legacyBIOS",
			args: &TemplateArgs{
				Name:                   "windows",
				UID:                    501,
				Home:                   "/home/foo.guest",
				Shell:                  "powershell.exe",
				SSHPubKeys:             []string{"ssh-rsa dummy foo@example.com"},
				Arch:                   limatype.X8664,
				User:                   "windows-user",
				WindowsInitialPassword: "dummy-password",
				LegacyBIOS:             true,
				TPM:                    true,
				IsWindowsServer:        true,
			},
			expectedAutounattendStrings: []string{
				`<Path>E:\viostor\2k25\amd64</Path>`,
				`<Username>windows-user</Username>`,
				`<Value>dummy-password</Value>`,
				`<Label>BIOS</Label>`,
				`<Value>1</Value>`,
			},
			expectedFirstLogonStrings: []string{
				`$logfile = "C:\Users\windows-user\lima-setup.log"`,
			},
		},
		{
			name: "disable TPM on Windows 11",
			args: &TemplateArgs{
				Name:                   "windows",
				UID:                    501,
				Home:                   "/home/foo.guest",
				Shell:                  "powershell.exe",
				SSHPubKeys:             []string{"ssh-rsa dummy foo@example.com"},
				Arch:                   limatype.X8664,
				User:                   "windows-user",
				WindowsInitialPassword: "dummy-password",
				TPM:                    false,
			},
			expectedAutounattendStrings: []string{
				`<Path>E:\viostor\w11\amd64</Path>`,
				`<Username>windows-user</Username>`,
				`<Value>dummy-password</Value>`,
				`<Type>EFI</Type>`,
				`BypassTPMCheck`,
				`<Value>6</Value>`,
			},
			expectedFirstLogonStrings: []string{
				`$logfile = "C:\Users\windows-user\lima-setup.log"`,
			},
		},
		{
			name: "Windows 11 arm64",
			args: &TemplateArgs{
				Name:                   "windows",
				UID:                    501,
				Home:                   "/home/foo.guest",
				Shell:                  "powershell.exe",
				SSHPubKeys:             []string{"ssh-rsa dummy foo@example.com"},
				Arch:                   limatype.AARCH64,
				User:                   "windows-user",
				WindowsInitialPassword: "dummy-password",
				TPM:                    false,
			},
			expectedAutounattendStrings: []string{
				`<Path>E:\viostor\w11\arm64</Path>`,
				`<Username>windows-user</Username>`,
				`<Value>dummy-password</Value>`,
				`<Type>EFI</Type>`,
				`BypassTPMCheck`,
				`<Value>3</Value>`,
			},
			expectedFirstLogonStrings: []string{
				`$logfile = "C:\Users\windows-user\lima-setup.log"`,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := ExecuteTemplateWindowsISO(tt.args)
			assert.NilError(t, err)
			for _, f := range layout {
				b, err := io.ReadAll(f.Reader)
				s := string(b)
				assert.NilError(t, err)
				switch f.Path {
				case "autounattend.xml":
					for _, expected := range tt.expectedAutounattendStrings {
						assert.Assert(t, strings.Contains(s, expected), fmt.Sprintf("expected: %s", expected))
					}
				case "first_logon.ps1":
					for _, expected := range tt.expectedFirstLogonStrings {
						assert.Assert(t, strings.Contains(s, expected), fmt.Sprintf("expected: %s", expected))
					}
				}
			}
		})
	}
}
