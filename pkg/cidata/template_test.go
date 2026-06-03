// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"encoding/base64"
	"encoding/xml"
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

func TestAutounattend(t *testing.T) {
	key := "ssh-rsa dummy foo@example.com"
	args := &TemplateArgs{
		Name:                    "default",
		User:                    "lima",
		TimeZone:                "UTC",
		WindowsComputerName:     "LIMA-DEFAULT",
		SSHPubKeys:              []string{key},
		WindowsSSHPubKeysBase64: base64.StdEncoding.EncodeToString([]byte(key)),
	}
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			output, err := ExecuteTemplateAutounattend(args, arch)
			assert.NilError(t, err)

			assert.Assert(t, xml.Unmarshal(output, new(any)) == nil, "output is not valid XML")
			s := string(output)
			assert.Assert(t, strings.Contains(s, "Microsoft-Windows-Setup"), "missing disk/install component")
			assert.Assert(t, strings.Contains(s, "Microsoft-Windows-Shell-Setup"), "missing shell setup component")
			assert.Assert(t, strings.Contains(s, "OpenSSH.Server"), "missing OpenSSH server capability")
			assert.Assert(t, strings.Contains(s, "administrators_authorized_keys"), "missing SSH authorized_keys setup")
			assert.Assert(t, strings.Contains(s, args.WindowsSSHPubKeysBase64), "missing SSH public key payload")
			assert.Assert(t, !strings.Contains(s, "PLACEHOLDER"), "must not contain placeholder SSH keys")
			assert.Assert(t, !strings.Contains(s, "ProductKey>VK7JG"), "must not contain generic product keys")
		})
	}
}
