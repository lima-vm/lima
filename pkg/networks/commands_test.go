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

func TestVDESock(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	vdeSock := config.VDESock("foo")
	assert.Equal(t, vdeSock, "/private/var/run/lima/foo.ctl")
}

func TestPIDFile(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	pidFile := config.PIDFile("name", "daemon")
	assert.Equal(t, pidFile, "/private/var/run/lima/name_daemon.pid")
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

	user, err := config.User(Switch)
	assert.NilError(t, err)
	assert.Equal(t, user.User, "daemon")
	assert.Equal(t, user.Group, config.Group)
	if runtime.GOOS == "darwin" {
		assert.Equal(t, user.Uid, uint32(1))
	}

	user, err = config.User(VMNet)
	assert.NilError(t, err)
	assert.Equal(t, user.User, "root")
	if runtime.GOOS == "darwin" {
		assert.Equal(t, user.Group, "wheel")
	} else {
		assert.Equal(t, user.Group, "root")
	}
	assert.Equal(t, user.Uid, uint32(0))
	assert.Equal(t, user.Gid, uint32(0))
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

	cmd := config.StartCmd("shared", Switch)
	assert.Equal(t, cmd, "/opt/vde/bin/vde_switch --pidfile=/private/var/run/lima/shared_switch.pid "+
		"--sock=/private/var/run/lima/shared.ctl --group=everyone --dirmode=0770 --nostdin")

	cmd = config.StartCmd("shared", VMNet)
	assert.Equal(t, cmd, "/opt/vde/bin/vde_vmnet --pidfile=/private/var/run/lima/shared_vmnet.pid --vde-group=everyone --vmnet-mode=shared "+
		"--vmnet-gateway=192.168.105.1 --vmnet-dhcp-end=192.168.105.254 --vmnet-mask=255.255.255.0 /private/var/run/lima/shared.ctl")

	cmd = config.StartCmd("bridged", VMNet)
	assert.Equal(t, cmd, "/opt/vde/bin/vde_vmnet --pidfile=/private/var/run/lima/bridged_vmnet.pid --vde-group=everyone --vmnet-mode=bridged "+
		"--vmnet-interface=en0 /private/var/run/lima/bridged.ctl")
}

func TestStopCmd(t *testing.T) {
	config, err := DefaultConfig()
	assert.NilError(t, err)

	cmd := config.StopCmd("name", "daemon")
	assert.Equal(t, cmd, "/usr/bin/pkill -F /private/var/run/lima/name_daemon.pid")
}
