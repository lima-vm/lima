// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// The daemons are passed in, so the grants are asserted on every host,
// including the ones where they are not installed.
func TestSudoersRendering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the rendered paths are POSIX paths")
	}
	cfg := Config{
		Paths: Paths{VarRun: "/run/lima"},
		Group: "staff",
		Networks: map[string]Network{
			"user-v2": {Mode: ModeUserV2, Gateway: net.ParseIP("192.168.104.1"), NetMask: net.ParseIP("255.255.255.0")},
			"shared": {
				Mode:    ModeShared,
				Gateway: net.ParseIP("192.168.105.1"),
				DHCPEnd: net.ParseIP("192.168.105.254"),
				NetMask: net.ParseIP("255.255.255.0"),
			},
			"bridged": {Mode: ModeBridged, Interface: "br0"},
		},
	}

	t.Run("lima-privileged-net", func(t *testing.T) {
		helper := filepath.Join(t.TempDir(), LimaPrivilegedNet)
		assert.NilError(t, os.WriteFile(helper, nil, 0o755))
		root := osutil.User{User: "root", Group: "root"}

		got, err := cfg.sudoers(false, []sudoersDaemon{{name: LimaPrivilegedNet, path: helper, user: root}})
		assert.NilError(t, err)
		// sha256 of the empty helper written above
		const digest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		want := strings.ReplaceAll(`# Manage "bridged" network daemons

%staff ALL=(root:root) NOPASSWD:NOSETENV: \
    DIGEST HELPER start --pidfile=/run/lima/bridged_lima-privileged-net.pid --mode=bridged --bridge=br0, \
    DIGEST HELPER tap --bridge=br0 --network=bridged -- *, \
    /usr/bin/pkill -F /run/lima/bridged_lima-privileged-net.pid

# Manage "shared" network daemons

%staff ALL=(root:root) NOPASSWD:NOSETENV: \
    DIGEST HELPER start --pidfile=/run/lima/shared_lima-privileged-net.pid --mode=shared --bridge=lima-shared --gateway=192.168.105.1 --dhcp-end=192.168.105.254 --netmask=255.255.255.0, \
    DIGEST HELPER tap --bridge=lima-shared --network=shared -- *, \
    /usr/bin/pkill -F /run/lima/shared_lima-privileged-net.pid
`, "DIGEST HELPER", digest+" "+helper)
		assert.Equal(t, got, want)
	})

	t.Run("socket_vmnet", func(t *testing.T) {
		const helper = "/opt/socket_vmnet/bin/socket_vmnet"
		root := osutil.User{User: "root", Group: "wheel"}

		got, err := cfg.sudoers(true, []sudoersDaemon{{name: SocketVMNet, path: helper, user: root}})
		assert.NilError(t, err)
		want := `%staff ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /run/lima

# Manage "bridged" network daemons

%staff ALL=(root:wheel) NOPASSWD:NOSETENV: \
    /opt/socket_vmnet/bin/socket_vmnet --pidfile=/run/lima/bridged_socket_vmnet.pid --socket-group=staff --vmnet-mode=bridged --vmnet-interface=br0 /run/lima/socket_vmnet.bridged, \
    /usr/bin/pkill -F /run/lima/bridged_socket_vmnet.pid

# Manage "shared" network daemons

%staff ALL=(root:wheel) NOPASSWD:NOSETENV: \
    /opt/socket_vmnet/bin/socket_vmnet --pidfile=/run/lima/shared_socket_vmnet.pid --socket-group=staff --vmnet-mode=shared --vmnet-gateway=192.168.105.1 --vmnet-dhcp-end=192.168.105.254 --vmnet-mask=255.255.255.0 /run/lima/socket_vmnet.shared, \
    /usr/bin/pkill -F /run/lima/shared_socket_vmnet.pid
`
		assert.Equal(t, got, want)
	})
}
