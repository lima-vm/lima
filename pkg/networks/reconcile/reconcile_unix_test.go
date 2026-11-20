//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package reconcile

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet"
)

const externalConfig = `networks:
  ext:
    mode: user-v2
    external: true
    gateway: 192.168.200.1
    netmask: 255.255.255.0
`

func TestMain(m *testing.M) {
	// usernet.Start() runs `<executable> usernet` as the daemon, which is this test binary here.
	// Exit right away, as running the tests again would start yet another one.
	if len(os.Args) > 1 && os.Args[1] == "usernet" {
		os.Exit(1)
	}
	m.Run()
}

func TestExternalUsernet(t *testing.T) {
	// networks.LoadConfig() caches the configuration, so the subtests share one LIMA_HOME and depend on their order.
	//nolint:usetesting // t.TempDir() may return a path too long for a Unix socket.
	limaHome, err := os.MkdirTemp("", "lima")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(limaHome) })
	t.Setenv("LIMA_HOME", limaHome)
	assert.NilError(t, os.MkdirAll(filepath.Join(limaHome, "_config"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(limaHome, "_config", "networks.yaml"), []byte(externalConfig), 0o644))

	cfg, err := networks.LoadConfig()
	assert.NilError(t, err)
	external, err := cfg.External("ext")
	assert.NilError(t, err)
	assert.Assert(t, external)

	nwDir := filepath.Join(limaHome, "_networks", "ext")
	assert.NilError(t, os.MkdirAll(nwDir, 0o755))
	sock := func(sockType usernet.SockType) string {
		s, err := usernet.Sock("ext", sockType)
		assert.NilError(t, err)
		return s
	}
	assertNotSpawned := func(t *testing.T) {
		t.Helper()
		for _, f := range []string{"usernet.ext.stdout.log", "usernet.ext.stderr.log"} {
			_, err := os.Stat(filepath.Join(nwDir, f))
			assert.Assert(t, os.IsNotExist(err), "Lima must not spawn a daemon for an external network, found %s", f)
		}
	}

	t.Run("not used by the new instance", func(t *testing.T) {
		// The daemon is not running, but the instance that is about to be started is not attached to the network.
		assert.NilError(t, startNetwork(t.Context(), &cfg, "ext", nil, false))
		assertNotSpawned(t)
	})

	t.Run("daemon absent", func(t *testing.T) {
		err := startNetwork(t.Context(), &cfg, "ext", nil, true)
		assert.ErrorContains(t, err, "external usernet network `ext` is not running")
		assertNotSpawned(t)
		// usernet.Start() must not start a daemon either.
		assert.ErrorContains(t, usernet.Start(t.Context(), "ext"), "is not running")
		assertNotSpawned(t)
	})

	t.Run("stale sockets", func(t *testing.T) {
		// A daemon that is killed leaves its sockets behind.
		for _, sockType := range []usernet.SockType{usernet.FDSock, usernet.QEMUSock, usernet.EndpointSock} {
			var lc net.ListenConfig
			l, err := lc.Listen(t.Context(), "unix", sock(sockType))
			assert.NilError(t, err)
			l.(*net.UnixListener).SetUnlinkOnClose(false)
			assert.NilError(t, l.Close())
		}
		err := startNetwork(t.Context(), &cfg, "ext", nil, true)
		for _, sockType := range []usernet.SockType{usernet.FDSock, usernet.QEMUSock, usernet.EndpointSock} {
			assert.NilError(t, os.Remove(sock(sockType)))
		}
		assert.ErrorContains(t, err, "external usernet network `ext` is not responding")
		assertNotSpawned(t)
	})

	t.Run("endpoint only", func(t *testing.T) {
		// A daemon that only serves vz guests has no qemu socket. The data socket is dialed by the driver,
		// so only the endpoint is required here.
		var lc net.ListenConfig
		l, err := lc.Listen(t.Context(), "unix", sock(usernet.EndpointSock))
		assert.NilError(t, err)
		srv := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/services/dhcp/leases" {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write([]byte("{}"))
			}),
			ReadHeaderTimeout: time.Second,
		}
		go func() { _ = srv.Serve(l) }()
		defer func() { _ = srv.Close() }()
		for _, sockType := range []usernet.SockType{usernet.FDSock, usernet.QEMUSock} {
			_, err := os.Stat(sock(sockType))
			assert.Assert(t, os.IsNotExist(err))
		}
		assert.NilError(t, startNetwork(t.Context(), &cfg, "ext", nil, true))
		assertNotSpawned(t)
	})

	// The netstack of Lima stands in for the external daemon.
	ctx := t.Context()
	assert.NilError(t, usernet.StartGVisorNetstack(ctx, &usernet.GVisorNetstackOpts{
		MTU:        1500,
		QemuSocket: sock(usernet.QEMUSock),
		FdSocket:   sock(usernet.FDSock),
		Endpoint:   sock(usernet.EndpointSock),
		Subnet:     "192.168.200.0/24",
		Async:      true,
	}))

	t.Run("daemon present", func(t *testing.T) {
		assert.NilError(t, startNetwork(t.Context(), &cfg, "ext", nil, true))
		assertNotSpawned(t)
	})

	t.Run("not stopped", func(t *testing.T) {
		// No instance is using the network, so Reconcile() would write the leases and
		// stop the process in the PID file if the network was not external.
		cmd := exec.CommandContext(t.Context(), "sleep", "60")
		assert.NilError(t, cmd.Start())
		exited := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(exited)
		}()
		t.Cleanup(func() { _ = cmd.Process.Kill() })
		pidFile, err := usernet.PIDFile("ext")
		assert.NilError(t, err)
		assert.NilError(t, os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644))

		assert.NilError(t, Reconcile(t.Context(), ""))
		assert.NilError(t, usernet.Stop(t.Context(), "ext"))

		signaled := false
		select {
		case <-exited:
			signaled = true
		case <-time.After(500 * time.Millisecond):
		}
		assert.Assert(t, !signaled, "Lima signaled the external daemon")
		leases, err := usernet.Leases("ext")
		assert.NilError(t, err)
		_, err = os.Stat(leases)
		assert.Assert(t, os.IsNotExist(err), "Lima must not write the leases of an external network")
		_, err = os.Stat(pidFile)
		assert.NilError(t, err, "Lima must not remove the PID file of an external network")
	})
}
