// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"net"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// TestPseudoLoopbackPacketConnKeepsForwardingAfterNonLoopbackDatagram checks
// that pseudoLoopbackPacketConn drops a datagram from a non-loopback address
// and keeps forwarding datagrams from loopback clients.
func TestPseudoLoopbackPacketConnKeepsForwardingAfterNonLoopbackDatagram(t *testing.T) {
	nonLoopbackIP := nonLoopbackIPv4(t)

	var lc net.ListenConfig
	echo, err := lc.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	assert.NilError(t, err)
	defer echo.Close()
	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := echo.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = echo.WriteTo(buf[:n], addr)
		}
	}()

	udpConn, err := lc.ListenPacket(t.Context(), "udp", "0.0.0.0:0")
	assert.NilError(t, err)
	defer udpConn.Close()
	port := udpConn.LocalAddr().(*net.UDPAddr).Port
	var dialer net.Dialer
	go HandleUDPConnection(t.Context(), dialer.DialContext, &pseudoLoopbackPacketConn{udpConn}, echo.LocalAddr().String())

	send := func(fromIP net.IP, msg string) (string, error) {
		conn, err := net.DialUDP("udp", &net.UDPAddr{IP: fromIP}, &net.UDPAddr{IP: fromIP, Port: port})
		assert.NilError(t, err)
		defer conn.Close()
		_, err = conn.Write([]byte(msg))
		assert.NilError(t, err)
		assert.NilError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		buf := make([]byte, 1500)
		n, err := conn.Read(buf)
		return string(buf[:n]), err
	}

	reply, err := send(net.IPv4(127, 0, 0, 1), "first")
	assert.NilError(t, err)
	assert.Equal(t, reply, "first")

	_, err = send(nonLoopbackIP, "non-loopback")
	assert.ErrorContains(t, err, "timeout")

	reply, err = send(net.IPv4(127, 0, 0, 1), "second")
	assert.NilError(t, err)
	assert.Equal(t, reply, "second")
}

func nonLoopbackIPv4(t *testing.T) net.IP {
	addrs, err := net.InterfaceAddrs()
	assert.NilError(t, err)
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
			return ipNet.IP
		}
	}
	t.Skip("host has no non-loopback IPv4 address")
	return nil
}
