package samba

import (
	"testing"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/templateutil"
	"gotest.tools/v3/assert"
)

func TestSMBConfTemplate(t *testing.T) {
	args := smbConfTmplArgs{
		StateDir:      "/tmp/foo",
		SMBPasswdFile: "/tmp/foo/smbpasswd",
		Username:      "dummyuser",
		Mounts: []limayaml.Mount{
			{
				Location: "/some/mount",
			},
			{
				Location: "/another/mount",
				Writable: true,
			},
		},
	}

	res, err := templateutil.Execute(smbConfTmpl, args)
	assert.NilError(t, err)
	t.Log(string(res))
}
