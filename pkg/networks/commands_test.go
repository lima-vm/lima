// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
)

func TestCheck(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	for _, name := range []string{"bridged", "shared", "host"} {
		err = config.Check(name)
		assert.NilError(t, err)
	}
	err = config.Check("unknown")
	assert.ErrorContains(t, err, "not defined")
}

func TestLogFile(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	logFile := config.LogFile("name", "daemon", "stream")
	networksDir, err := dirnames.LimaNetworksDir()
	assert.NilError(t, err)
	assert.Equal(t, logFile, filepath.Join(networksDir, "name_daemon.stream.log"))
}

func TestUser(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)
	if runtime.GOOS == "windows" {
		// unimplemented
		t.Skip()
	}

	t.Run("socket_vmnet", func(t *testing.T) {
		if ok, _ := config.IsDaemonInstalled(SocketVMNet); !ok {
			t.Skip("socket_vmnet is not installed")
		}
		user, err := config.User(SocketVMNet)
		assert.NilError(t, err)
		assert.Equal(t, user.User, "root")
		if runtime.GOOS == "darwin" {
			assert.Equal(t, user.Group, "wheel")
		} else {
			assert.Equal(t, user.Group, "root")
		}
		assert.Equal(t, user.Uid, uint32(0))
		assert.Equal(t, user.Gid, uint32(0))
	})
}

func TestMkdirCmd(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	cmd := config.MkdirCmd()
	assert.Equal(t, cmd, "/bin/mkdir -m 775 -p "+config.Paths.VarRun)
}

func TestStartCmd(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	varRunDir := config.Paths.VarRun

	t.Run("socket_vmnet", func(t *testing.T) {
		if ok, _ := config.IsDaemonInstalled(SocketVMNet); !ok {
			t.Skip("socket_vmnet is not installed")
		}

		cmd := config.StartCmd("shared", SocketVMNet)
		assert.Equal(t, cmd, "/opt/socket_vmnet/bin/socket_vmnet --pidfile="+filepath.Join(varRunDir, "shared_socket_vmnet.pid")+" --socket-group=admin --vmnet-mode=shared "+
			"--vmnet-gateway=192.168.105.1 --vmnet-dhcp-end=192.168.105.254 --vmnet-mask=255.255.255.0 "+filepath.Join(varRunDir, "socket_vmnet.shared"))

		cmd = config.StartCmd("bridged", SocketVMNet)
		assert.Equal(t, cmd, "/opt/socket_vmnet/bin/socket_vmnet --pidfile="+filepath.Join(varRunDir, "bridged_socket_vmnet.pid")+" --socket-group=admin --vmnet-mode=bridged "+
			"--vmnet-interface=en0 "+filepath.Join(varRunDir, "socket_vmnet.bridged"))
	})

	t.Run("lima-privileged-net", func(t *testing.T) {
		// The helper path is passed in, so the rendering is asserted on every host,
		// including the ones where the helper is not installed.
		const helper = "/usr/local/libexec/lima/privileged/lima-privileged-net"

		cmd := config.startCmd("shared", LimaPrivilegedNet, helper)
		assert.Equal(t, cmd, helper+" start --pidfile="+filepath.Join(varRunDir, "shared_lima-privileged-net.pid")+" --mode=shared --bridge=lima-shared "+
			"--gateway=192.168.105.1 --dhcp-end=192.168.105.254 --netmask=255.255.255.0")

		cmd = config.startCmd("bridged", LimaPrivilegedNet, helper)
		assert.Equal(t, cmd, helper+" start --pidfile="+filepath.Join(varRunDir, "bridged_lima-privileged-net.pid")+" --mode=bridged --bridge="+config.Networks["bridged"].Interface)

		assert.Equal(t, config.tapCmd(helper, "shared", "default"),
			helper+" tap --bridge=lima-shared --network=shared default")
		assert.Equal(t, config.tapCmd(helper, "shared", "*"),
			helper+" tap --bridge=lima-shared --network=shared *")
	})
}

func TestTapName(t *testing.T) {
	tap := TapName(1000, "default", "shared")
	assert.Equal(t, len(tap), 15)
	assert.Assert(t, IsTapName(tap))
	assert.Assert(t, TapName(1000, "default", "host") != tap)
	assert.Assert(t, TapName(1001, "default", "shared") != tap)
	assert.Assert(t, !IsTapName("eth0"))
	assert.Assert(t, !IsTapName("limatapzzzzzzzz"))
}

func TestStopCmd(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	cmd := config.StopCmd("name", "daemon")
	assert.Equal(t, cmd, "/usr/bin/pkill -F "+filepath.Join(config.Paths.VarRun, "name_daemon.pid"))
}

func TestIsManagedBridge(t *testing.T) {
	assert.Assert(t, IsManagedBridge("lima-shared"))
	assert.Assert(t, !IsManagedBridge("br0"))
}

func TestIsTapName(t *testing.T) {
	assert.Assert(t, len(tapPrefix)+tapDigits <= maxIfNameLen)
	assert.Assert(t, IsTapName("limatap01234567"))
	assert.Assert(t, !IsTapName("eth0"))
	assert.Assert(t, !IsTapName("limatapzzzzzzzz"))
}
