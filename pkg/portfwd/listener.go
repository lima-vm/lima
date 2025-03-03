// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	guestagentclient "github.com/lima-vm/lima/pkg/guestagent/api/client"
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

func (p *ClosableListeners) Forward(ctx context.Context, client *guestagentclient.GuestAgentClient,
	protocol string, hostAddress string, guestAddress string,
) {
	switch protocol {
	case "tcp", "tcp6":
		go p.forwardTCP(ctx, client, hostAddress, guestAddress)
	case "udp", "udp6":
		go p.forwardUDP(ctx, client, hostAddress, guestAddress)
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

func (p *ClosableListeners) forwardTCP(ctx context.Context, client *guestagentclient.GuestAgentClient, hostAddress, guestAddress string) {
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
			logrus.Errorf("failed to accept TCP connection: %v", err)
			if strings.Contains(err.Error(), "pseudoloopback") {
				// don't stop forwarding because the forwarder has rejected a non-local address
				continue
			}
			return
		}
		go HandleTCPConnection(ctx, client, conn, guestAddress)
	}
}

func (p *ClosableListeners) forwardUDP(ctx context.Context, client *guestagentclient.GuestAgentClient, hostAddress, guestAddress string) {
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

	HandleUDPConnection(ctx, client, udpConn, guestAddress)
}

func key(protocol, hostAddress, guestAddress string) string {
	return fmt.Sprintf("%s-%s-%s", protocol, hostAddress, guestAddress)
}
