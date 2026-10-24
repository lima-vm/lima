//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"

	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/portfwdserver"
)

func TestForwardUnixSocketIsOwnerOnly(t *testing.T) {
	oldUmask := syscall.Umask(0o002)
	t.Cleanup(func() { syscall.Umask(oldUmask) })

	dir := shortUnixTempDir(t)
	guestSocket := filepath.Join(dir, "guest.sock")
	hostSocket := filepath.Join(dir, "host.sock")

	echoUnixListener(t, guestSocket)

	client := newTestGuestAgentClient(t, &tunnelGuestService{tunnel: portfwdserver.NewTunnelServer()})
	defer client.Close()
	getClient := func() *guestagentclient.GuestAgentClient { return client }

	fw := NewPortForwarder(nil, false, false, nil)
	defer fw.Close()
	fw.ForwardStatic(t.Context(), DialContextToGRPCTunnel(getClient), "unix", hostSocket, guestSocket)

	conn := dialWithRetry(t, hostSocket)
	conn.Close()

	fi, err := os.Lstat(hostSocket)
	assert.NilError(t, err)
	assert.Equal(t, fi.Mode().Perm(), os.FileMode(0o600))

	// the private directory used for binding must not be left behind
	entries, err := os.ReadDir(dir)
	assert.NilError(t, err)
	for _, e := range entries {
		assert.Assert(t, !e.IsDir(), "unexpected leftover directory %s", e.Name())
	}
}
