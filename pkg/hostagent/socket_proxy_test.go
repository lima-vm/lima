// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/bicopy"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// mockForwardFn returns a forwardFn mock that creates real Unix socket relays.
// Tracks listeners in a map so cancel properly cleans up (prevents goroutine leaks).
func mockForwardFn(guestSocket string) func(context.Context, *ssh.SSHConfig,
	string, int, string, string, string, bool) error {
	var mu sync.Mutex
	listeners := make(map[string]net.Listener)
	return func(
		ctx context.Context, _ *ssh.SSHConfig, _ string,
		_ int, local, _, verb string, _ bool,
	) error {
		mu.Lock()
		defer mu.Unlock()
		if verb == verbCancel {
			if ln, ok := listeners[local]; ok {
				ln.Close()
				delete(listeners, local)
			}
			os.RemoveAll(local)
			return nil
		}
		// Create a Unix socket relay from local → guestSocket.
		ln, err := (&net.ListenConfig{}).Listen(ctx, "unix", local)
		if err != nil {
			return err
		}
		listeners[local] = ln
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				guestConn, err := (&net.Dialer{}).DialContext(ctx, "unix", guestSocket)
				if err != nil {
					conn.Close()
					continue
				}
				go bicopy.Bicopy(conn, guestConn, ctx.Done())
			}
		}()
		return nil
	}
}

// startEchoServer creates a Unix socket echo server and registers cleanup via t.Cleanup.
func startEchoServer(t *testing.T, socketPath string) {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socketPath)
	assert.NilError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buf := make([]byte, 4096)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					if _, err := conn.Write(buf[:n]); err != nil {
						return
					}
				}
			}()
		}
	}()
}

// newTestProxy creates a SocketForwardProxy with a mock forwardFn for testing.
func newTestProxy(t *testing.T, guestSocket string, autoPauseMgr *AutoPauseManager) (proxy *SocketForwardProxy, hostSocket string) {
	t.Helper()
	// Use /tmp for host socket to stay within macOS UnixPathMax (104 bytes).
	hostDir, err := os.MkdirTemp("/tmp", "lima-test-proxy-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(hostDir) })
	hostSocket = filepath.Join(hostDir, "host.sock")

	rules := []limatype.PortForward{
		{
			GuestSocket: guestSocket,
			HostSocket:  hostSocket,
		},
	}

	proxy = NewSocketForwardProxy(rules, autoPauseMgr, nil, func() (string, int) {
		return "127.0.0.1", 22
	})
	proxy.forwardFn = mockForwardFn(guestSocket)
	return proxy, hostSocket
}

func TestSocketProxy_ConstructorFiltersRules(t *testing.T) {
	rules := []limatype.PortForward{
		{GuestSocket: "/var/run/docker.sock", HostSocket: "/tmp/test-docker.sock"},
		{GuestSocket: "/var/run/reverse.sock", HostSocket: "/tmp/test-reverse.sock", Reverse: true},
		{HostSocket: "/tmp/tcp-only.sock"}, // no GuestSocket
		{GuestSocket: "/var/run/podman.sock", HostSocket: "/tmp/test-podman.sock"},
	}

	proxy := NewSocketForwardProxy(rules, nil, nil, func() (string, int) {
		return "127.0.0.1", 22
	})

	// Should only keep non-reverse socket rules with GuestSocket set.
	assert.Equal(t, len(proxy.rules), 2)
	assert.Equal(t, proxy.rules[0].guestSocket, "/var/run/docker.sock")
	assert.Equal(t, proxy.rules[1].guestSocket, "/var/run/podman.sock")
}

func TestSocketProxy_ForwardsWhenRunning(t *testing.T) {
	// Setup: mock guest echo server.
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	// Create proxy (no auto-pause manager — VM is always running).
	proxy, hostSocket := newTestProxy(t, guestSocket, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() { proxy.Close() })

	// Wait for internal tunnel to be ready.
	time.Sleep(100 * time.Millisecond)

	// Connect to host socket, send data, expect echo.
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)
	defer conn.Close()

	msg := []byte("hello from test")
	_, err = conn.Write(msg)
	assert.NilError(t, err)

	buf := make([]byte, len(msg))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf[:n]), "hello from test")
}

func TestSocketProxy_ResumesOnConnection(t *testing.T) {
	// Setup: mock guest echo server.
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	// Create auto-pause manager with paused VM.
	mock := &mockPausable{paused: true}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	proxy, hostSocket := newTestProxy(t, guestSocket, mgr)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() { proxy.Close() })

	// Resume the VM after a short delay (simulate real resume).
	go func() {
		time.Sleep(150 * time.Millisecond)
		mock.mu.Lock()
		mock.paused = false
		mock.mu.Unlock()
	}()

	// Connect to host socket — should block until resume completes.
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)
	defer conn.Close()

	msg := []byte("after resume")
	_, err = conn.Write(msg)
	assert.NilError(t, err)

	buf := make([]byte, len(msg))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf[:n]), "after resume")
}

func TestSocketProxy_ConcurrentConnections(t *testing.T) {
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	proxy, hostSocket := newTestProxy(t, guestSocket, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() { proxy.Close() })

	time.Sleep(100 * time.Millisecond)

	// Launch 10 concurrent connections.
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
			if err != nil {
				t.Errorf("connection %d failed: %v", id, err)
				return
			}
			defer conn.Close()

			msg := []byte("concurrent")
			if _, err := conn.Write(msg); err != nil {
				t.Errorf("write %d failed: %v", id, err)
				return
			}

			buf := make([]byte, len(msg))
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buf)
			if err != nil {
				t.Errorf("read %d failed: %v", id, err)
				return
			}
			if string(buf[:n]) != "concurrent" {
				t.Errorf("connection %d: got %q, want %q", id, string(buf[:n]), "concurrent")
			}
		}(i)
	}
	wg.Wait()
}

func TestSocketProxy_ReEstablishesTunnel(t *testing.T) {
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	proxy, hostSocket := newTestProxy(t, guestSocket, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() { proxy.Close() })

	time.Sleep(100 * time.Millisecond)

	// Verify initial tunnel works.
	conn1, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)
	msg := []byte("before kill")
	_, err = conn1.Write(msg)
	assert.NilError(t, err)
	buf := make([]byte, len(msg))
	_ = conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn1.Read(buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf[:n]), "before kill")
	conn1.Close()

	// Kill the internal tunnel by cancelling it via forwardFn.
	assert.Assert(t, len(proxy.rules) > 0)
	rule := proxy.rules[0]
	sshAddr, sshPort := proxy.sshAddressPort()
	err = proxy.forwardFn(ctx, proxy.sshConfig, sshAddr, sshPort,
		rule.internalSocket, rule.guestSocket, verbCancel, false)
	assert.NilError(t, err)

	// Connect again — ensureTunnel should detect stale tunnel and re-establish.
	conn2, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)
	defer conn2.Close()

	msg2 := []byte("after re-establish")
	_, err = conn2.Write(msg2)
	assert.NilError(t, err)

	buf2 := make([]byte, len(msg2))
	_ = conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	n2, err := conn2.Read(buf2)
	assert.NilError(t, err)
	assert.Equal(t, string(buf2[:n2]), "after re-establish")
}

func TestSocketProxy_CleanShutdown(t *testing.T) {
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	proxy, hostSocket := newTestProxy(t, guestSocket, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Open a connection to keep a handleConn goroutine active.
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)

	// Close the proxy — should shut down cleanly.
	err = proxy.Close()
	assert.NilError(t, err)

	// Connection should be broken.
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	assert.Assert(t, readErr != nil, "connection should be broken after Close")
	conn.Close()

	// Host socket file should be removed.
	_, statErr := os.Stat(hostSocket)
	assert.Assert(t, os.IsNotExist(statErr), "host socket file should be removed after Close")

	// Temp directories should be removed.
	for _, dir := range proxy.tmpDirs {
		_, statErr := os.Stat(dir)
		assert.Assert(t, os.IsNotExist(statErr), "temp dir %s should be removed after Close", dir)
	}
}

func TestSocketProxy_DoubleClose(t *testing.T) {
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	proxy, _ := newTestProxy(t, guestSocket, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)

	// First close should succeed.
	err = proxy.Close()
	assert.NilError(t, err)

	// Second close should be a no-op (not panic or error).
	err = proxy.Close()
	assert.NilError(t, err)
}

func TestSocketProxy_RefreshTunnels(t *testing.T) {
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	proxy, hostSocket := newTestProxy(t, guestSocket, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() { proxy.Close() })

	time.Sleep(100 * time.Millisecond)

	// Verify initial connection works.
	conn1, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)
	msg := []byte("before refresh")
	_, err = conn1.Write(msg)
	assert.NilError(t, err)
	buf := make([]byte, len(msg))
	_ = conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn1.Read(buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf[:n]), "before refresh")
	conn1.Close()

	// RefreshTunnels cancels and re-establishes all SSH tunnels.
	proxy.RefreshTunnels(ctx)

	// Verify connection still works after refresh.
	time.Sleep(100 * time.Millisecond)
	conn2, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)
	defer conn2.Close()

	msg2 := []byte("after refresh")
	_, err = conn2.Write(msg2)
	assert.NilError(t, err)

	buf2 := make([]byte, len(msg2))
	_ = conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	n2, err := conn2.Read(buf2)
	assert.NilError(t, err)
	assert.Equal(t, string(buf2[:n2]), "after refresh")
}

func TestSocketProxy_TimeoutWhenResumeHangs(t *testing.T) {
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })
	guestSocket := filepath.Join(guestDir, "guest.sock")
	startEchoServer(t, guestSocket)

	// Create auto-pause manager with permanently paused VM (never resumes).
	mock := &mockPausable{paused: true}
	mgr := NewAutoPauseManager(mock, 1*time.Hour, 5*time.Second, DefaultIdleSignalConfig())

	proxy, hostSocket := newTestProxy(t, guestSocket, mgr)

	// Use a short-lived context so the WaitForRunning timeout is short.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = proxy.Start(ctx)
	assert.NilError(t, err)
	t.Cleanup(func() { proxy.Close() })

	// Connect — handleConn will call WaitForRunning which blocks.
	// Cancel the parent context to break WaitForRunning quickly (instead of waiting 30s).
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)

	// The connection should be broken because WaitForRunning timed out.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	assert.Assert(t, readErr != nil, "connection should fail when resume times out")
	conn.Close()
}

// --- Active Connection Tracking Tests ---

func TestSocketProxy_ActiveConnectionCount(t *testing.T) {
	proxy, hostSocket, guestSocket := setupActiveConnsProxy(t)
	defer proxy.Close()

	// No connections initially.
	assert.Assert(t, !proxy.HasActiveConnections(), "should have no active connections initially")
	assert.Equal(t, proxy.ActiveConnectionCount(), int64(0))

	// Connect first client.
	conn1 := dialAndVerify(t, hostSocket, guestSocket)

	// Wait for the connection to be tracked.
	waitForConns(t, proxy, 1)
	assert.Assert(t, proxy.HasActiveConnections(), "should have active connection after connect")
	assert.Equal(t, proxy.ActiveConnectionCount(), int64(1))

	// Connect second client.
	conn2 := dialAndVerify(t, hostSocket, guestSocket)
	waitForConns(t, proxy, 2)
	assert.Equal(t, proxy.ActiveConnectionCount(), int64(2))

	// Close first client.
	conn1.Close()
	waitForConns(t, proxy, 1)
	assert.Equal(t, proxy.ActiveConnectionCount(), int64(1))

	// Close second client.
	conn2.Close()
	waitForConns(t, proxy, 0)
	assert.Assert(t, !proxy.HasActiveConnections(), "should have no active connections after all close")
}

func TestSocketProxy_HasActiveConnections(t *testing.T) {
	proxy, hostSocket, guestSocket := setupActiveConnsProxy(t)
	defer proxy.Close()

	assert.Assert(t, !proxy.HasActiveConnections())

	conn := dialAndVerify(t, hostSocket, guestSocket)
	waitForConns(t, proxy, 1)
	assert.Assert(t, proxy.HasActiveConnections())

	conn.Close()
	waitForConns(t, proxy, 0)
	assert.Assert(t, !proxy.HasActiveConnections())
}

func TestSocketProxy_ActiveConnsPreventPause(t *testing.T) {
	// Integration: wire proxy's HasActiveConnections to IdleTracker BusyCheck.
	proxy, hostSocket, guestSocket := setupActiveConnsProxy(t)
	defer proxy.Close()

	tracker := NewIdleTracker(50 * time.Millisecond)
	tracker.AddBusyCheck("active-connections", proxy.HasActiveConnections)

	// No connections — idle after timeout.
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "should be idle with no connections")

	// Connect — busy-check keeps tracker awake.
	conn := dialAndVerify(t, hostSocket, guestSocket)
	waitForConns(t, proxy, 1)
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, !tracker.IsIdle(), "should not be idle with active connection")

	// Disconnect — idle after timeout.
	conn.Close()
	waitForConns(t, proxy, 0)
	time.Sleep(60 * time.Millisecond)
	assert.Assert(t, tracker.IsIdle(), "should be idle after connection closes")
}

// setupActiveConnsProxy creates a proxy with a real guest socket echo server.
func setupActiveConnsProxy(t *testing.T) (proxy *SocketForwardProxy, hostSocket, guestSocket string) {
	t.Helper()

	// Use /tmp for sockets to avoid macOS path length limits.
	guestDir, err := os.MkdirTemp("/tmp", "lima-test-guest-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(guestDir) })

	hostDir, err := os.MkdirTemp("/tmp", "lima-test-host-") //nolint:usetesting // /tmp required for macOS UnixPathMax (104 bytes)
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(hostDir) })

	// Create a guest socket echo server.
	guestSocket = filepath.Join(guestDir, "g.sock")
	guestLn, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", guestSocket)
	assert.NilError(t, err)
	t.Cleanup(func() { guestLn.Close() })
	go func() {
		for {
			conn, err := guestLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	hostSocket = filepath.Join(hostDir, "h.sock")
	rules := []limatype.PortForward{
		{GuestSocket: guestSocket, HostSocket: hostSocket},
	}
	proxy = NewSocketForwardProxy(rules, nil, nil, func() (string, int) { return "", 0 })
	proxy.forwardFn = mockForwardFn(guestSocket)
	err = proxy.Start(t.Context())
	assert.NilError(t, err)
	return proxy, hostSocket, guestSocket
}

// dialAndVerify connects to the proxy and verifies the echo relay works.
func dialAndVerify(t *testing.T, hostSocket, _ string) net.Conn {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", hostSocket)
	assert.NilError(t, err)

	// Verify echo.
	msg := []byte("ping")
	_, err = conn.Write(msg)
	assert.NilError(t, err)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	assert.NilError(t, err)
	assert.Equal(t, string(buf), "ping")
	_ = conn.SetReadDeadline(time.Time{}) // clear deadline
	return conn
}

// waitForConns polls until the proxy reaches the expected active connection count.
func waitForConns(t *testing.T, proxy *SocketForwardProxy, expected int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if proxy.ActiveConnectionCount() == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, proxy.ActiveConnectionCount(), expected, "timed out waiting for activeConns")
}
