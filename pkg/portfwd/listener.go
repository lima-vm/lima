// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/osutil"
)

type ClosableListeners struct {
	listenConfig   net.ListenConfig
	listeners      map[string]net.Listener
	udpListeners   map[string]net.PacketConn
	listenersRW    sync.Mutex
	udpListenersRW sync.Mutex
}

func NewClosableListener() *ClosableListeners {
	listenConfig := net.ListenConfig{
		Control: Control,
	}

	return &ClosableListeners{
		listeners:    make(map[string]net.Listener),
		udpListeners: make(map[string]net.PacketConn),
		listenConfig: listenConfig,
	}
}

func (p *ClosableListeners) Close() error {
	p.listenersRW.Lock()
	defer p.listenersRW.Unlock()
	var errs []error
	for _, listener := range p.listeners {
		if err := listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	clear(p.listeners)
	p.udpListenersRW.Lock()
	defer p.udpListenersRW.Unlock()
	for _, listener := range p.udpListeners {
		if err := listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	clear(p.udpListeners)
	return errors.Join(errs...)
}

func (p *ClosableListeners) Forward(ctx context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error),
	protocol string, hostAddress string, guestAddress string,
) {
	switch protocol {
	case "tcp":
		go p.forwardTCP(ctx, dialContext, hostAddress, guestAddress)
	case "udp":
		go p.forwardUDP(ctx, dialContext, hostAddress, guestAddress)
	case "unix":
		go p.forwardUnix(ctx, dialContext, hostAddress, guestAddress)
	}
}

func (p *ClosableListeners) Remove(_ context.Context, protocol, hostAddress, guestAddress string) {
	logrus.Debugf("removing listener for hostAddress: %s, guestAddress: %s", hostAddress, guestAddress)
	key := key(protocol, hostAddress, guestAddress)
	switch protocol {
	case "tcp", "tcp6", "unix":
		p.listenersRW.Lock()
		defer p.listenersRW.Unlock()
		listener, ok := p.listeners[key]
		if ok {
			listener.Close()
			delete(p.listeners, key)
		}
	case "udp":
		p.udpListenersRW.Lock()
		defer p.udpListenersRW.Unlock()
		listener, ok := p.udpListeners[key]
		if ok {
			listener.Close()
			delete(p.udpListeners, key)
		}
	}
}

func (p *ClosableListeners) forwardTCP(ctx context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error), hostAddress, guestAddress string) {
	key := key("tcp", hostAddress, guestAddress)

	p.listenersRW.Lock()
	_, ok := p.listeners[key]
	if ok {
		p.listenersRW.Unlock()
		return
	}
	tcpLis, err := Listen(ctx, p.listenConfig, hostAddress)
	if err != nil {
		logListenError(err, "tcp", hostAddress)
		p.listenersRW.Unlock()
		return
	}
	defer p.Remove(ctx, "tcp", hostAddress, guestAddress)
	p.listeners[key] = tcpLis
	p.listenersRW.Unlock()
	for {
		conn, err := tcpLis.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("failed to accept TCP connection: %v", err)
			if strings.Contains(err.Error(), "pseudoloopback") {
				// don't stop forwarding because the forwarder has rejected a non-local address
				continue
			}
			return
		}
		go HandleTCPConnection(ctx, dialContext, conn, guestAddress)
	}
}

func (p *ClosableListeners) forwardUnix(ctx context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error), hostAddress, guestAddress string) {
	key := key("unix", hostAddress, guestAddress)

	p.listenersRW.Lock()
	_, ok := p.listeners[key]
	if ok {
		p.listenersRW.Unlock()
		return
	}
	unixLis, err := Listen(ctx, p.listenConfig, hostAddress)
	if err != nil {
		logListenError(err, "unix", hostAddress)
		p.listenersRW.Unlock()
		return
	}
	defer p.Remove(ctx, "unix", hostAddress, guestAddress)
	p.listeners[key] = unixLis
	p.listenersRW.Unlock()
	for {
		conn, err := unixLis.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("failed to accept unix connection: %v", err)
			if strings.Contains(err.Error(), "pseudoloopback") {
				// don't stop forwarding because the forwarder has rejected a non-local address
				continue
			}
			return
		}
		go handleUnixConnection(ctx, dialContext, conn, guestAddress)
	}
}

func handleUnixConnection(ctx context.Context, dialContext func(ctx context.Context, network, addr string) (net.Conn, error), conn net.Conn, guestAddress string) {
	defer conn.Close()
	dst, err := dialContext(ctx, "unix", guestAddress)
	if err != nil {
		logrus.WithError(err).Errorf("could not dial guest unix socket %s", guestAddress)
		return
	}
	defer dst.Close()

	// On a copy error, tear down both directions so neither goroutine stays blocked.
	// The tunnel is cancelled with CloseRead rather than Close, because Close calls
	// gRPC CloseSend, which must not run concurrently with the writer goroutine.
	var abortOnce sync.Once
	abort := func() {
		abortOnce.Do(func() {
			_ = conn.Close()
			if cr, ok := dst.(interface{ CloseRead() error }); ok {
				_ = cr.CloseRead()
			}
		})
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(dst, conn); err != nil {
			abort()
			return
		}
		closeWriteIfPossible(dst)
	}()
	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, dst); err != nil {
			abort()
			return
		}
		closeWriteIfPossible(conn)
	}()
	wg.Wait()
	logrus.Debugf("unix proxy for guestAddr: %s closed", guestAddress)
}

func closeWriteIfPossible(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}

func (p *ClosableListeners) forwardUDP(ctx context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error), hostAddress, guestAddress string) {
	key := key("udp", hostAddress, guestAddress)

	p.udpListenersRW.Lock()
	_, ok := p.udpListeners[key]
	if ok {
		p.udpListenersRW.Unlock()
		return
	}

	udpConn, err := ListenPacket(ctx, p.listenConfig, hostAddress)
	if err != nil {
		logListenError(err, "udp", hostAddress)
		p.udpListenersRW.Unlock()
		return
	}
	defer p.Remove(ctx, "udp", hostAddress, guestAddress)
	p.udpListeners[key] = udpConn
	p.udpListenersRW.Unlock()

	HandleUDPConnection(ctx, dialContext, udpConn, guestAddress)
}

func key(protocol, hostAddress, guestAddress string) string {
	if protocol == "unix" {
		guestAddress = ""
	}
	return fmt.Sprintf("%s-%s-%s", protocol, hostAddress, guestAddress)
}

func prepareUnixSocket(hostSocket string) error {
	if err := osutil.RemoveStaleSocket(hostSocket); err != nil {
		return fmt.Errorf("can't clean up %#q: %w", hostSocket, err)
	}
	if err := os.MkdirAll(filepath.Dir(hostSocket), 0o755); err != nil {
		return fmt.Errorf("can't create directory for local socket %#q: %w", hostSocket, err)
	}
	return nil
}

// listenUnixOwnerOnly listens on a Unix socket that only the owner can access (0600),
// matching SSH's default StreamLocalBindMask, independent of the process umask.
// The socket is bound inside a private (0700) temporary directory and renamed into
// place, so it is never reachable by other users with looser permissions.
func listenUnixOwnerOnly(ctx context.Context, hostSocket string) (net.Listener, error) {
	var lc net.ListenConfig
	if runtime.GOOS == "windows" {
		return lc.Listen(ctx, "unix", hostSocket)
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(hostSocket), ".l")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	tmpSocket := filepath.Join(tmpDir, "s")
	lis, err := lc.Listen(ctx, "unix", tmpSocket)
	if err != nil {
		return nil, err
	}
	unixLis := lis.(*net.UnixListener)
	// The socket is unlinked by ownerOnlyUnixListener.Close at its final path instead.
	unixLis.SetUnlinkOnClose(false)
	if err := os.Chmod(tmpSocket, 0o600); err != nil {
		_ = unixLis.Close()
		return nil, err
	}
	if err := os.Rename(tmpSocket, hostSocket); err != nil {
		_ = unixLis.Close()
		return nil, err
	}
	return &ownerOnlyUnixListener{UnixListener: unixLis, path: hostSocket}, nil
}

type ownerOnlyUnixListener struct {
	*net.UnixListener
	path string
}

func (l *ownerOnlyUnixListener) Close() error {
	err := l.UnixListener.Close()
	if rmErr := osutil.RemoveStaleSocket(l.path); rmErr != nil {
		logrus.WithError(rmErr).Debugf("failed to remove unix socket %s", l.path)
	}
	return err
}

func logListenError(err error, proto, hostAddress string) {
	var negligibleReason string
	if proto != "unix" {
		if _, portStr, err := net.SplitHostPort(hostAddress); err == nil {
			switch proto {
			case "tcp":
				//nolint:gocritic // singleCaseSwitch: should rewrite switch statement to if statement
				switch portStr {
				case "53":
					negligibleReason = "DNS port (often conflicts with the system resolver)"
				}
			case "udp":
				switch portStr {
				case "53":
					negligibleReason = "DNS port (often conflicts with the system resolver)"
				case "323":
					negligibleReason = "NTP port (often conflicts with the system NTP server)"
				case "5353":
					negligibleReason = "mDNS port (often conflicts with the system mDNS responder)"
				case "5355":
					negligibleReason = "LLMNR port (often conflicts with the system LLMNR responder)"
				}
			}
		}
		if osutil.IsEACCES(err) {
			negligibleReason = "privileged port"
		}
	}

	if negligibleReason != "" {
		logrus.WithError(err).WithField("negligible-reason", negligibleReason).Debugf("failed to listen %s: %s", proto, hostAddress)
		logrus.Infof("Not forwarding %s %s", strings.ToUpper(proto), hostAddress)
		return
	}

	logrus.WithError(err).Warnf("failed to listen %s: %s", proto, hostAddress)
}
