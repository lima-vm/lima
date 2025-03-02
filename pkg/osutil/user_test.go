/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
