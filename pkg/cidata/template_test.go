package cidata

import (
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestTemplate(t *testing.T) {
	args := TemplateArgs{
		Name: "default",
		User: "foo",
		UID:  501,
		Home: "/home/foo.linux",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []Mount{
			{MountPoint: "/Users/dummy"},
			{MountPoint: "/Users/dummy/lima"},
		},
		MountType: "reverse-sshfs",
	}
	layout, _, err := ExecuteTemplate(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %q ===", f.Path)
		b, err := io.ReadAll(f.Reader)
		assert.NilError(t, err)
		t.Log(string(b))
		if f.Path == "user-data" {
			// mounted later
			assert.Assert(t, !strings.Contains(string(b), "mounts:"))
		}
	}
}

func TestTemplate9p(t *testing.T) {
	args := TemplateArgs{
		Name: "default",
		User: "foo",
		UID:  501,
		Home: "/home/foo.linux",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []Mount{
			{Tag: "mount0", MountPoint: "/Users/dummy", Type: "9p", Options: "ro,trans=virtio"},
			{Tag: "mount1", MountPoint: "/Users/dummy/lima", Type: "9p", Options: "rw,trans=virtio"},
		},
		MountType: "9p",
	}
	layout, _, err := ExecuteTemplate(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %q ===", f.Path)
		b, err := io.ReadAll(f.Reader)
		assert.NilError(t, err)
		t.Log(string(b))
		if f.Path == "user-data" {
			// mounted at boot
			assert.Assert(t, strings.Contains(string(b), "mounts:"))
		}
	}
}
