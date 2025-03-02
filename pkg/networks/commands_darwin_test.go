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

package networks

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSock(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	sock := config.Sock("foo")
	assert.Equal(t, sock, "/private/var/run/lima/socket_vmnet.foo")
}

func TestPIDFile(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	pidFile := config.PIDFile("name", "daemon")
	assert.Equal(t, pidFile, "/private/var/run/lima/name_daemon.pid")
}
