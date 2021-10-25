package cidata

import (
	"io"
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
	}
	layout, err := ExecuteTemplate(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %q ===", f.Path)
		b, err := io.ReadAll(f.Reader)
		assert.NilError(t, err)
		t.Log(string(b))
	}
}
