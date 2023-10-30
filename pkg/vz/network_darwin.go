//go:build darwin && !no_vz
// +build darwin,!no_vz

package vz

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"time"

	"github.com/balajiv113/fd"

	"github.com/sirupsen/logrus"
	"inet.af/tcpproxy"
)

func PassFDToUnix(unixSock string) (*os.File, error) {
	unixConn, err := net.Dial("unix", unixSock)
	if err != nil {
		return nil, err
	}

	server, client, err := createSockPair()
	if err != nil {
		return nil, err
	}
	err = fd.Put(unixConn.(*net.UnixConn), server)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// DialQemu support connecting to QEMU supported network stack via unix socket
// Returns os.File, connected dgram connection to be used for vz
func DialQemu(unixSock string) (*os.File, error) {
	unixConn, err := net.Dial("unix", unixSock)
	if err != nil {
		return nil, err
	}
	qemuConn := &QEMUPacketConn{unixConn: unixConn}

	server, client, err := createSockPair()
	if err != nil {
		return nil, err
	}
	dgramConn, err := net.FileConn(server)
	if err != nil {
		return nil, err
	}

	remote := tcpproxy.DialProxy{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dgramConn, nil
		},
	}
	go remote.HandleConn(qemuConn)

	return client, nil
}

// QEMUPacketConn converts raw network packet to a QEMU supported network packet.
type QEMUPacketConn struct {
	unixConn net.Conn
}

var _ net.Conn = (*QEMUPacketConn)(nil)

// Read gets rid of the QEMU header packet and returns the raw packet as response
func (v *QEMUPacketConn) Read(b []byte) (n int, err error) {
	header := make([]byte, 4)
	_, err = io.ReadFull(v.unixConn, header)
	if err != nil {
		logrus.Errorln("Failed to read header", err)
	}

	size := binary.BigEndian.Uint32(header)
	reader := io.LimitReader(v.unixConn, int64(size))
	_, err = reader.Read(b)

	if err != nil {
		logrus.Errorln("Failed to read packet", err)
	}
	return int(size), nil
}

// Write puts QEMU header packet first and then writes the raw packet
func (v *QEMUPacketConn) Write(b []byte) (n int, err error) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(b)))
	_, err = v.unixConn.Write(header)
	if err != nil {
		logrus.Errorln("Failed to write header", err)
	}

	write, err := v.unixConn.Write(b)
	if err != nil {
		logrus.Errorln("Failed to write packet", err)
	}
	return write, nil
}

func (v *QEMUPacketConn) Close() error {
	return v.unixConn.Close()
}

func (v *QEMUPacketConn) LocalAddr() net.Addr {
	return v.unixConn.LocalAddr()
}

func (v *QEMUPacketConn) RemoteAddr() net.Addr {
	return v.unixConn.RemoteAddr()
}

func (v *QEMUPacketConn) SetDeadline(t time.Time) error {
	return v.unixConn.SetDeadline(t)
}

func (v *QEMUPacketConn) SetReadDeadline(t time.Time) error {
	return v.unixConn.SetReadDeadline(t)
}

func (v *QEMUPacketConn) SetWriteDeadline(t time.Time) error {
	return v.unixConn.SetWriteDeadline(t)
}
