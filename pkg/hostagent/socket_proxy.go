// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/bicopy"
	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// SocketForwardProxy manages host-side Unix socket proxies that transparently
// resume the VM when clients connect during auto-pause. It replaces direct
// SSH -L tunneling for socket forwarding when auto-pause is enabled.
//
// For each portForward rule with a guestSocket (excluding reverse rules):
//   - Listens on the user-facing hostSocket path
//   - Forwards to an internal SSH tunnel socket in /tmp (short path)
//   - Calls Touch() on incoming connections to trigger VM resume
//   - Re-establishes stale SSH tunnels after resume
//
// This is service-agnostic: works for Docker, containerd, Podman, or any
// Unix socket forwarded through Lima.
type SocketForwardProxy struct {
	rules          []socketRule
	autoPauseMgr   *AutoPauseManager
	sshConfig      *ssh.SSHConfig
	sshAddressPort func() (string, int)
	listeners      []net.Listener
	mu             sync.Mutex      // protects tunnel state and forwardSSH calls
	closing        atomic.Bool     // set during shutdown to prevent new tunnels
	closeCtx       context.Context //nolint:containedctx // lifecycle-bound context cancelled in Close()
	closeCancel    context.CancelFunc
	wg             sync.WaitGroup // tracks active handleConn goroutines
	activeConns    atomic.Int64   // number of open bicopy relay sessions
	connSem        chan struct{}  // semaphore limiting concurrent proxy connections
	tmpDirs        []string       // temp directories to clean up on Close
	// forwardFn is the tunnel establishment function. Defaults to forwardSSH
	// in production. Tests inject a mock that creates a Unix socket relay
	// in-process.
	forwardFn func(ctx context.Context, sshConfig *ssh.SSHConfig, sshAddress string,
		sshPort int, local, remote, verb string, reverse bool) error
}

type socketRule struct {
	hostSocket     string // user-facing path (e.g., ~/.lima/inst/sock/docker.sock)
	internalSocket string // short tunnel endpoint (e.g., /tmp/lima-sp-xxxx/sock)
	guestSocket    string // path inside VM (e.g., /var/run/docker.sock)
}

// NewSocketForwardProxy creates a proxy from portForward rules.
// Filters to socket rules with GuestSocket != "" and Reverse == false.
// Reverse socket forwarding is architecturally incompatible (guest creates the
// listener) and falls through to direct SSH -R forwarding.
func NewSocketForwardProxy(
	rules []limatype.PortForward,
	autoPauseMgr *AutoPauseManager,
	sshConfig *ssh.SSHConfig,
	sshAddressPort func() (string, int),
) *SocketForwardProxy {
	var filtered []socketRule
	for _, rule := range rules {
		if rule.GuestSocket == "" {
			continue
		}
		if rule.Reverse {
			logrus.Warnf("Socket proxy: skipping reverse socket rule %s (not supported by proxy)", rule.GuestSocket)
			continue
		}
		filtered = append(filtered, socketRule{
			hostSocket:  hostAddress(rule, &api.IPPort{}),
			guestSocket: rule.GuestSocket,
		})
	}
	return &SocketForwardProxy{
		rules:          filtered,
		autoPauseMgr:   autoPauseMgr,
		sshConfig:      sshConfig,
		sshAddressPort: sshAddressPort,
		connSem:        make(chan struct{}, 256),
		forwardFn:      forwardSSH,
	}
}

// HasActiveConnections reports whether any client connections are being relayed.
func (p *SocketForwardProxy) HasActiveConnections() bool {
	return p.activeConns.Load() > 0
}

// ActiveConnectionCount returns the current number of open relay sessions.
func (p *SocketForwardProxy) ActiveConnectionCount() int64 {
	return p.activeConns.Load()
}

// Start creates listeners on host sockets and establishes initial SSH tunnels.
func (p *SocketForwardProxy) Start(ctx context.Context) error {
	p.closeCtx, p.closeCancel = context.WithCancel(ctx)

	for i := range p.rules {
		rule := &p.rules[i]

		// Create temp directory for internal socket (short path, <30 bytes).
		dir, err := os.MkdirTemp("/tmp", "lima-sp-")
		if err != nil {
			p.rollbackStart()
			return fmt.Errorf("failed to create temp dir for socket proxy: %w", err)
		}
		p.tmpDirs = append(p.tmpDirs, dir)
		rule.internalSocket = filepath.Join(dir, "sock")

		// Establish initial SSH tunnel to internal socket.
		sshAddr, sshPort := p.sshAddressPort()
		if err := p.forwardFn(ctx, p.sshConfig, sshAddr, sshPort,
			rule.internalSocket, rule.guestSocket, verbForward, false); err != nil {
			logrus.WithError(err).Warnf("Socket proxy: initial tunnel setup failed for %s", rule.guestSocket)
			// Don't fail — ensureTunnel will retry on first connection.
		}

		// Create parent directory for host socket (e.g., ~/.lima/inst/sock/).
		if err := os.MkdirAll(filepath.Dir(rule.hostSocket), 0o750); err != nil {
			p.rollbackStart()
			return fmt.Errorf("failed to create directory for host socket %q: %w", rule.hostSocket, err)
		}
		// Remove stale socket file if it exists.
		os.RemoveAll(rule.hostSocket)

		// Start listening on the user-facing host socket.
		ln, err := (&net.ListenConfig{}).Listen(ctx, "unix", rule.hostSocket)
		if err != nil {
			p.rollbackStart()
			return fmt.Errorf("failed to listen on %s: %w", rule.hostSocket, err)
		}
		p.listeners = append(p.listeners, ln)

		logrus.Debugf("Socket proxy: listening on %s → %s (tunnel: %s)",
			rule.hostSocket, rule.guestSocket, rule.internalSocket)

		// Start accept loop in a goroutine. Uses p.closeCtx (not ctx) so
		// Close() can interrupt WaitForRunning and ensureTunnel via cancel.
		go p.serve(p.closeCtx, ln, *rule)
	}
	return nil
}

// rollbackStart cleans up resources allocated during a failed Start().
func (p *SocketForwardProxy) rollbackStart() {
	// Cancel closeCtx first — any already-spawned serve goroutines may have
	// accepted connections and started handleConn, which blocks on
	// WaitForRunning(closeCtx). Without this, those goroutines hang until
	// the parent ctx is cancelled.
	if p.closeCancel != nil {
		p.closeCancel()
	}
	for _, ln := range p.listeners {
		ln.Close()
	}
	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sshAddr, sshPort := p.sshAddressPort()
	for _, rule := range p.rules {
		if rule.internalSocket != "" {
			_ = p.forwardFn(cancelCtx, p.sshConfig, sshAddr, sshPort,
				rule.internalSocket, rule.guestSocket, verbCancel, false)
		}
		// Remove host socket file created by the listener.
		os.RemoveAll(rule.hostSocket)
	}
	for _, dir := range p.tmpDirs {
		os.RemoveAll(dir)
	}
}

// Close shuts down the proxy gracefully.
func (p *SocketForwardProxy) Close() error {
	// 1. Prevent new tunnels from being established.
	//    Also serves as a double-close guard — if already set, we've been called before.
	if p.closing.Swap(true) {
		return nil
	}

	// 2. Close all listeners (stops accept loops, no new connections).
	for _, ln := range p.listeners {
		ln.Close()
	}

	// 3. Cancel closeCtx to break in-flight bicopy goroutines.
	//    Without this, wg.Wait() could hang forever on paused VM connections.
	p.closeCancel()

	// 4. Wait for active connections to drain, with a hard timeout.
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// Clean shutdown — all goroutines exited.
	case <-time.After(10 * time.Second):
		logrus.Warn("Socket proxy: shutdown timeout, forcing close")
	}

	// 5. Cancel all SSH tunnels (with timeout to prevent hanging).
	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sshAddr, sshPort := p.sshAddressPort()
	for _, rule := range p.rules {
		_ = p.forwardFn(cancelCtx, p.sshConfig, sshAddr, sshPort,
			rule.internalSocket, rule.guestSocket, verbCancel, false)
	}

	// 6. Remove temp directories (internal sockets).
	for _, dir := range p.tmpDirs {
		os.RemoveAll(dir)
	}

	// 7. Remove host socket files. Go's net.Listener.Close() does NOT
	//    remove Unix socket files automatically.
	for _, rule := range p.rules {
		os.RemoveAll(rule.hostSocket)
	}
	return nil
}

// serve runs the accept loop for a single socket rule.
func (p *SocketForwardProxy) serve(ctx context.Context, ln net.Listener, rule socketRule) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if p.closing.Load() {
				return // clean shutdown
			}
			logrus.WithError(err).Warnf("Socket proxy: accept error on %s", rule.hostSocket)
			return
		}
		// Limit concurrent connections to prevent resource exhaustion.
		select {
		case p.connSem <- struct{}{}:
		default:
			logrus.Warnf("Socket proxy: connection limit reached, rejecting connection on %s", rule.hostSocket)
			conn.Close()
			continue
		}
		p.wg.Go(func() {
			defer func() { <-p.connSem }()
			p.handleConn(ctx, conn, rule)
		})
	}
}

// handleConn handles a single client connection through the proxy.
func (p *SocketForwardProxy) handleConn(ctx context.Context, conn net.Conn, rule socketRule) {
	defer conn.Close()

	// 1. Signal activity to auto-pause manager (triggers resume if paused).
	if p.autoPauseMgr != nil {
		p.autoPauseMgr.Touch()

		// 2. Wait for the VM to be running (blocks until resume completes).
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := p.autoPauseMgr.WaitForRunning(waitCtx); err != nil {
			logrus.WithError(err).Warnf("Socket proxy: timed out waiting for VM resume for %s", rule.guestSocket)
			return
		}
	}

	// 3. Ensure SSH tunnel to internal socket is active.
	if err := p.ensureTunnel(ctx, rule); err != nil {
		logrus.WithError(err).Warnf("Socket proxy: failed to establish tunnel for %s", rule.guestSocket)
		return
	}

	// 4. Dial the internal tunnel socket.
	tunnelConn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: rule.internalSocket, Net: "unix"})
	if err != nil {
		logrus.WithError(err).Warnf("Socket proxy: failed to dial internal socket %s", rule.internalSocket)
		return
	}
	defer tunnelConn.Close()

	// Track active relay sessions. Increment AFTER DialUnix succeeds — failed
	// dials are not active connections. Decrement via defer when bicopy completes.
	p.activeConns.Add(1)
	defer p.activeConns.Add(-1)

	// 5. Bidirectional relay. Uses p.closeCtx so Close() can break the relay.
	bicopy.Bicopy(conn, tunnelConn, p.closeCtx.Done())
}

// ensureTunnel checks if the SSH tunnel for a rule is active and re-establishes
// it if stale. Thread-safe — serializes tunnel operations via p.mu.
func (p *SocketForwardProxy) ensureTunnel(ctx context.Context, rule socketRule) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Guard against shutdown race.
	if p.closing.Load() {
		return errors.New("proxy is shutting down")
	}

	// Quick check: can we connect to the internal socket?
	if p.isTunnelAlive(rule) {
		return nil
	}

	logrus.Infof("Socket proxy: (re-)establishing tunnel for %s → %s", rule.guestSocket, rule.internalSocket)

	// Cancel any stale tunnel (best-effort — may already be dead).
	sshAddr, sshPort := p.sshAddressPort()
	_ = p.forwardFn(ctx, p.sshConfig, sshAddr, sshPort,
		rule.internalSocket, rule.guestSocket, verbCancel, false)

	// Establish fresh tunnel.
	if err := p.forwardFn(ctx, p.sshConfig, sshAddr, sshPort,
		rule.internalSocket, rule.guestSocket, verbForward, false); err != nil {
		return fmt.Errorf("failed to establish tunnel %s → %s: %w",
			rule.guestSocket, rule.internalSocket, err)
	}

	// Wait for internal socket to be connectable (SSH -L creates it asynchronously).
	return p.waitForSocket(ctx, rule.internalSocket, 2*time.Second)
}

// isTunnelAlive checks if the internal socket exists and is connectable.
func (p *SocketForwardProxy) isTunnelAlive(rule socketRule) bool {
	conn, err := (&net.Dialer{Timeout: 50 * time.Millisecond}).DialContext(context.Background(), "unix", rule.internalSocket)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// waitForSocket polls until a Unix socket is connectable (not just existing).
// An os.Stat check is insufficient — SSH may create the socket file before it
// starts accepting connections. We use a fast dial probe instead.
func (p *SocketForwardProxy) waitForSocket(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("socket %s: %w", path, ctx.Err())
		default:
		}
		conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", path)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("socket %s not connectable within %s", path, timeout)
}

// RefreshTunnels cancels and re-establishes all SSH tunnels.
// Called after VM resume or system wake to pre-warm tunnels.
// SSH -O forward is idempotent — calling it multiple times succeeds.
func (p *SocketForwardProxy) RefreshTunnels(ctx context.Context) {
	for attempt := range 3 {
		p.mu.Lock()
		if p.closing.Load() {
			p.mu.Unlock()
			return
		}

		allOK := true
		sshAddr, sshPort := p.sshAddressPort()
		for _, rule := range p.rules {
			// Cancel stale tunnel (best-effort).
			_ = p.forwardFn(ctx, p.sshConfig, sshAddr, sshPort,
				rule.internalSocket, rule.guestSocket, verbCancel, false)
			// Re-establish.
			if err := p.forwardFn(ctx, p.sshConfig, sshAddr, sshPort,
				rule.internalSocket, rule.guestSocket, verbForward, false); err != nil {
				logrus.WithError(err).Warnf("Socket proxy: refresh attempt %d failed for %s",
					attempt+1, rule.guestSocket)
				allOK = false
			}
		}
		p.mu.Unlock()

		if allOK {
			return
		}
		if attempt < 2 {
			// Sleep WITHOUT holding the mutex — allows concurrent ensureTunnel()
			// calls to proceed while we wait between retry attempts.
			time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
		}
	}
	logrus.Warn("Socket proxy: tunnel refresh failed after 3 attempts; connections will retry on demand")
}
