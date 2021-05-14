package cidata

import (
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
	userData, err := GenerateUserData(args)
	assert.NilError(t, err)
	t.Log(string(userData))

	metaData, err := GenerateMetaData(args)
	assert.NilError(t, err)
	t.Log(string(metaData))
}
