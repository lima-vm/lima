package networks

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lima-vm/lima/pkg/store/dirnames"
	"gotest.tools/v3/assert"
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
	if runtime.GOOS != "darwin" && config.Group == "everyone" {
		// The "everyone" group is a specific macOS feature to include non-local accounts.
		config.Group = "staff"
	}
	if runtime.GOOS == "windows" {
		// unimplemented
		t.Skip()
	}

	t.Run("socket_vmnet", func(t *testing.T) {
		if ok, _ := config.IsDaemonInstalled(SocketVMNet); !ok {
			t.Skipf("socket_vmnet is not installed")
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

	t.Run("vde_vmnet", func(t *testing.T) {
		if ok, _ := config.IsDaemonInstalled(VDEVMNet); !ok {
			t.Skipf("vde_vmnet is not installed")
		}
		user, err := config.User(VDESwitch)
		assert.NilError(t, err)
		assert.Equal(t, user.User, "daemon")
		assert.Equal(t, user.Group, config.Group)
		if runtime.GOOS == "darwin" {
			assert.Equal(t, user.Uid, uint32(1))
		}

		user, err = config.User(VDEVMNet)
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
	assert.Equal(t, cmd, "/bin/mkdir -m 775 -p /private/var/run/lima")
}

func TestStartCmd(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	varRunDir := filepath.Join("/", "private", "var", "run", "lima")

	t.Run("socket_vmnet", func(t *testing.T) {
		if ok, _ := config.IsDaemonInstalled(SocketVMNet); !ok {
			t.Skipf("socket_vmnet is not installed")
		}

		cmd := config.StartCmd("shared", SocketVMNet)
		assert.Equal(t, cmd, "/opt/socket_vmnet/bin/socket_vmnet --pidfile="+filepath.Join(varRunDir, "shared_socket_vmnet.pid")+" --socket-group=everyone --vmnet-mode=shared "+
			"--vmnet-gateway=192.168.105.1 --vmnet-dhcp-end=192.168.105.254 --vmnet-mask=255.255.255.0 "+filepath.Join(varRunDir, "socket_vmnet.shared"))

		cmd = config.StartCmd("bridged", SocketVMNet)
		assert.Equal(t, cmd, "/opt/socket_vmnet/bin/socket_vmnet --pidfile="+filepath.Join(varRunDir, "bridged_socket_vmnet.pid")+" --socket-group=everyone --vmnet-mode=bridged "+
			"--vmnet-interface=en0 "+filepath.Join(varRunDir, "socket_vmnet.bridged"))
	})

	t.Run("vde_vmnet", func(t *testing.T) {
		if ok, _ := config.IsDaemonInstalled(VDEVMNet); !ok {
			t.Skipf("vde_vmnet is not installed")
		}
		cmd := config.StartCmd("shared", VDESwitch)
		assert.Equal(t, cmd, "/opt/vde/bin/vde_switch --pidfile="+filepath.Join(varRunDir, "shared_switch.pid")+" "+
			"--sock="+filepath.Join(varRunDir, "shared.ctl")+" --group=everyone --dirmode=0770 --nostdin")

		cmd = config.StartCmd("shared", VDEVMNet)
		assert.Equal(t, cmd, "/opt/vde/bin/vde_vmnet --pidfile="+filepath.Join(varRunDir, "shared_vmnet.pid")+" --vde-group=everyone --vmnet-mode=shared "+
			"--vmnet-gateway=192.168.105.1 --vmnet-dhcp-end=192.168.105.254 --vmnet-mask=255.255.255.0 "+filepath.Join(varRunDir, "shared.ctl"))

		cmd = config.StartCmd("bridged", VDEVMNet)
		assert.Equal(t, cmd, "/opt/vde/bin/vde_vmnet --pidfile="+filepath.Join(varRunDir, "bridged_vmnet.pid")+" --vde-group=everyone --vmnet-mode=bridged "+
			"--vmnet-interface=en0 "+filepath.Join(varRunDir, "bridged.ctl"))
	})
}

func TestStopCmd(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	varRunDir := filepath.Join("/", "private", "var", "run", "lima")

	cmd := config.StopCmd("name", "daemon")
	assert.Equal(t, cmd, "/usr/bin/pkill -F "+filepath.Join(varRunDir, "name_daemon.pid"))
}
