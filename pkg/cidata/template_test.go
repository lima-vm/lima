package cidata

import (
	"io/ioutil"
	"testing"

	"gotest.tools/v3/assert"
)

func TestTemplate(t *testing.T) {
	args := TemplateArgs{
		Name: "default",
		User: "foo",
		UID:  501,
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []string{
			"/Users/dummy",
			"/Users/dummy/lima",
		},
		MountsWritable: []bool{
			false,
			true,
		},
	}
	layout, err := ExecuteTemplate(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %q ===", f.Path)
		b, err := ioutil.ReadAll(f.Reader)
		assert.NilError(t, err)
		t.Log(string(b))
	}
}
