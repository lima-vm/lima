/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package portfwd

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/sirupsen/logrus"
)

func Listen(ctx context.Context, listenConfig net.ListenConfig, hostAddress string) (net.Listener, error) {
	localIPStr, localPortStr, _ := net.SplitHostPort(hostAddress)
	localIP := net.ParseIP(localIPStr)
	localPort, _ := strconv.Atoi(localPortStr)

	if !localIP.Equal(IPv4loopback1) || localPort >= 1024 {
		tcpLis, err := listenConfig.Listen(ctx, "tcp", hostAddress)
		if err != nil {
			logrus.Errorf("failed to listen tcp: %v", err)
			return nil, err
		}
		return tcpLis, nil
	}
	tcpLis, err := listenConfig.Listen(ctx, "tcp", fmt.Sprintf("0.0.0.0:%d", localPort))
	if err != nil {
		logrus.Errorf("failed to listen tcp: %v", err)
		return nil, err
	}
	return &pseudoLoopbackListener{tcpLis}, nil
}

func ListenPacket(ctx context.Context, listenConfig net.ListenConfig, hostAddress string) (net.PacketConn, error) {
	localIPStr, localPortStr, _ := net.SplitHostPort(hostAddress)
	localIP := net.ParseIP(localIPStr)
	localPort, _ := strconv.Atoi(localPortStr)

	if !localIP.Equal(IPv4loopback1) || localPort >= 1024 {
		udpConn, err := listenConfig.ListenPacket(ctx, "udp", hostAddress)
		if err != nil {
			logrus.Errorf("failed to listen udp: %v", err)
			return nil, err
		}
		return udpConn, nil
	}
	udpConn, err := listenConfig.ListenPacket(ctx, "udp", fmt.Sprintf("0.0.0.0:%d", localPort))
	if err != nil {
		logrus.Errorf("failed to listen udp: %v", err)
		return nil, err
	}
	return &pseudoLoopbackPacketConn{udpConn}, nil
}

type pseudoLoopbackListener struct {
	net.Listener
}

func (p pseudoLoopbackListener) Accept() (net.Conn, error) {
	conn, err := p.Listener.Accept()
	if err != nil {
		return nil, err
	}

	remoteAddr := conn.RemoteAddr().String() // ip:port
	remoteAddrIP, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		logrus.WithError(err).Debugf("pseudoloopback forwarder: rejecting non-loopback remoteAddr %q (unparsable)", remoteAddr)
		conn.Close()
		return nil, err
	}
	if !IsLoopback(remoteAddrIP) {
		err := fmt.Errorf("pseudoloopback forwarder: rejecting non-loopback remoteAddr %q", remoteAddr)
		logrus.Debug(err)
		conn.Close()
		return nil, err
	}
	logrus.Infof("pseudoloopback forwarder: accepting connection from %q", remoteAddr)
	return conn, nil
}

type pseudoLoopbackPacketConn struct {
	net.PacketConn
}

func (pk *pseudoLoopbackPacketConn) ReadFrom(bytes []byte) (n int, addr net.Addr, err error) {
	n, remoteAddr, err := pk.PacketConn.ReadFrom(bytes)
	if err != nil {
		return 0, nil, err
	}

	remoteAddrIP, _, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		return 0, nil, err
	}
	if !IsLoopback(remoteAddrIP) {
		return 0, nil, fmt.Errorf("pseudoloopback forwarder: rejecting non-loopback remoteAddr %q", remoteAddr)
	}
	return n, remoteAddr, nil
}

func (pk *pseudoLoopbackPacketConn) WriteTo(bytes []byte, remoteAddr net.Addr) (n int, err error) {
	remoteAddrIP, _, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		return 0, err
	}
	if !IsLoopback(remoteAddrIP) {
		return 0, fmt.Errorf("pseudoloopback forwarder: rejecting non-loopback remoteAddr %q", remoteAddr)
	}
	return pk.PacketConn.WriteTo(bytes, remoteAddr)
}

func IsLoopback(addr string) bool {
	return net.ParseIP(addr).IsLoopback()
}
