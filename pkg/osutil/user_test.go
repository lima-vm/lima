package osutil

import (
	"path"
	"strconv"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
)

const limaVersion = "1.0.0"

// "admin" is a reserved username in 1.0.0
func TestLimaUserAdminNew(t *testing.T) {
	currentUser.Username = "admin"
	once = new(sync.Once)
	user := LimaUser(limaVersion, false)
	assert.Equal(t, user.Username, fallbackUser)
}

// "admin" is allowed in older instances
func TestLimaUserAdminOld(t *testing.T) {
	currentUser.Username = "admin"
	once = new(sync.Once)
	user := LimaUser("0.23.0", false)
	assert.Equal(t, user.Username, "admin")
}

func TestLimaUserInvalid(t *testing.T) {
	currentUser.Username = "use@example.com"
	once = new(sync.Once)
	user := LimaUser(limaVersion, false)
	assert.Equal(t, user.Username, fallbackUser)
}

func TestLimaUserUid(t *testing.T) {
	currentUser.Username = fallbackUser
	once = new(sync.Once)
	user := LimaUser(limaVersion, false)
	_, err := strconv.Atoi(user.Uid)
	assert.NilError(t, err)
}

func TestLimaUserGid(t *testing.T) {
	currentUser.Username = fallbackUser
	once = new(sync.Once)
	user := LimaUser(limaVersion, false)
	_, err := strconv.Atoi(user.Gid)
	assert.NilError(t, err)
}

func TestLimaHomeDir(t *testing.T) {
	currentUser.Username = fallbackUser
	once = new(sync.Once)
	user := LimaUser(limaVersion, false)
	// check for absolute unix path (/home)
	assert.Assert(t, path.IsAbs(user.HomeDir), user.HomeDir)
}
