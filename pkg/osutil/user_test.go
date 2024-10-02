package osutil

import (
	"path"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLimaUserWarn(t *testing.T) {
	_, err := LimaUser(true)
	assert.NilError(t, err)
}

func TestLimaUsername(t *testing.T) {
	user, err := LimaUser(false)
	assert.NilError(t, err)
	// check for reasonable unix user name
	assert.Assert(t, ValidateUsername(user.Username), user.Username)
}

func TestLimaUserUid(t *testing.T) {
	user, err := LimaUser(false)
	assert.NilError(t, err)
	_, err = strconv.Atoi(user.Uid)
	assert.NilError(t, err)
}

func TestLimaUserGid(t *testing.T) {
	user, err := LimaUser(false)
	assert.NilError(t, err)
	_, err = strconv.Atoi(user.Gid)
	assert.NilError(t, err)
}

func TestLimaHomeDir(t *testing.T) {
	user, err := LimaUser(false)
	assert.NilError(t, err)
	// check for absolute unix path (/home)
	assert.Assert(t, path.IsAbs(user.HomeDir), user.HomeDir)
}
