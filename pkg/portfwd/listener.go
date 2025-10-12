// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
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
	case "tcp", "tcp6":
		go p.forwardTCP(ctx, dialContext, hostAddress, guestAddress)
	case "udp", "udp6":
		go p.forwardUDP(ctx, dialContext, hostAddress, guestAddress)
	}
}

func (p *ClosableListeners) Remove(_ context.Context, protocol, hostAddress, guestAddress string) {
	logrus.Debugf("removing listener for hostAddress: %s, guestAddress: %s", hostAddress, guestAddress)
	key := key(protocol, hostAddress, guestAddress)
	switch protocol {
	case "tcp", "tcp6":
		p.listenersRW.Lock()
		defer p.listenersRW.Unlock()
		listener, ok := p.listeners[key]
		if ok {
			listener.Close()
			delete(p.listeners, key)
		}
	case "udp", "udp6":
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
		logrus.Errorf("failed to listen to TCP connection: %v", err)
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

func (p *ClosableListeners) forwardUDP(ctx context.Context, dialContext func(ctx context.Context, network string, addr string) (net.Conn, error), hostAddress, guestAddress string) {
	key := key("udp", hostAddress, guestAddress)
	defer p.Remove(ctx, "udp", hostAddress, guestAddress)

	p.udpListenersRW.Lock()
	_, ok := p.udpListeners[key]
	if ok {
		p.udpListenersRW.Unlock()
		return
	}

	udpConn, err := ListenPacket(ctx, p.listenConfig, hostAddress)
	if err != nil {
		logrus.Errorf("failed to listen udp: %v", err)
		p.udpListenersRW.Unlock()
		return
	}
	p.udpListeners[key] = udpConn
	p.udpListenersRW.Unlock()

	HandleUDPConnection(ctx, dialContext, udpConn, guestAddress)
}

func key(protocol, hostAddress, guestAddress string) string {
	return fmt.Sprintf("%s-%s-%s", protocol, hostAddress, guestAddress)
}

func prepareUnixSocket(hostSocket string) error {
	if err := os.RemoveAll(hostSocket); err != nil {
		return fmt.Errorf("can't clean up %q: %w", hostSocket, err)
	}
	if err := os.MkdirAll(filepath.Dir(hostSocket), 0o755); err != nil {
		return fmt.Errorf("can't create directory for local socket %q: %w", hostSocket, err)
	}
	return nil
}
