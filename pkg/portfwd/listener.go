package portfwd

import (
	"context"
	"fmt"
	"net"
	"sync"

	guestagentclient "github.com/lima-vm/lima/pkg/guestagent/api/client"
	"github.com/sirupsen/logrus"
)

type ClosableListeners struct {
	listenConfig net.ListenConfig

	listenersMu sync.Mutex
	listeners   map[string]net.Listener

	udpListenersMu sync.Mutex
	udpListeners   map[string]net.PacketConn
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
	key := key(protocol, hostAddress, guestAddress)
	switch protocol {
	case "tcp", "tcp6":
		p.listenersMu.Lock()
		defer p.listenersMu.Unlock()
		listener, ok := p.listeners[key]
		if ok {
			listener.Close()
			delete(p.listeners, key)
		}
	case "udp", "udp6":
		p.udpListenersMu.Lock()
		defer p.udpListenersMu.Unlock()
		listener, ok := p.udpListeners[key]
		if ok {
			listener.Close()
			delete(p.udpListeners, key)
		}
	}
}

func (p *ClosableListeners) forwardTCP(ctx context.Context, client *guestagentclient.GuestAgentClient, hostAddress, guestAddress string) {
	key := key("tcp", hostAddress, guestAddress)
	defer p.Remove(ctx, "tcp", hostAddress, guestAddress)

	p.listenersMu.Lock()
	_, ok := p.listeners[key]
	if ok {
		p.listenersMu.Unlock()
		return
	}
	tcpLis, err := Listen(ctx, p.listenConfig, hostAddress)
	if err != nil {
		logrus.Errorf("failed to accept TCP connection: %v", err)
		p.listenersMu.Unlock()
		return
	}
	p.listeners[key] = tcpLis
	p.listenersMu.Unlock()
	for {
		conn, err := tcpLis.Accept()
		if err != nil {
			logrus.Errorf("failed to accept TCP connection: %v", err)
			return
		}
		go HandleTCPConnection(ctx, client, conn, guestAddress)
	}
}

func (p *ClosableListeners) forwardUDP(ctx context.Context, client *guestagentclient.GuestAgentClient, hostAddress, guestAddress string) {
	key := key("udp", hostAddress, guestAddress)
	defer p.Remove(ctx, "udp", hostAddress, guestAddress)

	p.udpListenersMu.Lock()
	_, ok := p.udpListeners[key]
	if ok {
		p.udpListenersMu.Unlock()
		return
	}

	udpConn, err := ListenPacket(ctx, p.listenConfig, hostAddress)
	if err != nil {
		logrus.Errorf("failed to listen udp: %v", err)
		p.udpListenersMu.Unlock()
		return
	}
	p.udpListeners[key] = udpConn
	p.udpListenersMu.Unlock()

	HandleUDPConnection(ctx, client, udpConn, guestAddress)
}

func key(protocol, hostAddress, guestAddress string) string {
	return fmt.Sprintf("%s-%s-%s", protocol, hostAddress, guestAddress)
}
