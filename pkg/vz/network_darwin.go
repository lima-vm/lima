//go:build darwin && !no_vz

package vz

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/balajiv113/fd"

	"github.com/sirupsen/logrus"
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

// DialQemu support connecting to QEMU supported network stack via unix socket.
// Returns os.File, connected dgram connection to be used for vz.
func DialQemu(unixSock string) (*os.File, error) {
	unixConn, err := net.Dial("unix", unixSock)
	if err != nil {
		return nil, err
	}
	qemuConn := &qemuPacketConn{Conn: unixConn}

	server, client, err := createSockPair()
	if err != nil {
		return nil, err
	}
	dgramConn, err := net.FileConn(server)
	if err != nil {
		return nil, err
	}
	vzConn := &packetConn{Conn: dgramConn}

	go forwardPackets(qemuConn, vzConn)

	return client, nil
}

func forwardPackets(qemuConn *qemuPacketConn, vzConn *packetConn) {
	defer qemuConn.Close()
	defer vzConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(qemuConn, vzConn); err != nil {
			logrus.Errorf("Failed to forward packets from VZ to VMNET: %s", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(vzConn, qemuConn); err != nil {
			logrus.Errorf("Failed to forward packets from VMNET to VZ: %s", err)
		}
	}()

	wg.Wait()
}

// qemuPacketConn converts raw network packet to a QEMU supported network packet.
type qemuPacketConn struct {
	net.Conn
}

// Read reads a QEMU packet and returns the contained raw packet.  Returns (len,
// nil) if a packet was read, and (0, err) on error. Errors means the protocol
// is broken and the socket must be closed.
func (c *qemuPacketConn) Read(b []byte) (n int, err error) {
	var size uint32
	if err := binary.Read(c.Conn, binary.BigEndian, &size); err != nil {
		// Likely connection closed by peer.
		return 0, err
	}
	return io.ReadFull(c.Conn, b[:size])
}

// Write writes a QEMU packet containing the raw packet. Returns (len(b), nil)
// if a packet was written, and (0, err) if a packet was not fully written.
// Errors means the protocol is broken and the socket must be closed.
func (c *qemuPacketConn) Write(b []byte) (int, error) {
	size := len(b)
	header := uint32(size)
	if err := binary.Write(c.Conn, binary.BigEndian, header); err != nil {
		return 0, err
	}

	start := 0
	for start < size {
		nw, err := c.Conn.Write(b[start:])
		if err != nil {
			return 0, err
		}
		start += nw
	}
	return size, nil
}

// Testing show that retries are very rare (e.g 24 of 62,499,008 packets) and
// requires 1 or 2 retries to complete the write. A 100 microseconds sleep loop
// consumes about 4% CPU on M1 Pro.
const writeRetryDelay = 100 * time.Microsecond

// packetConn handles ENOBUFS errors when writing to unixgram socket.
type packetConn struct {
	net.Conn
}

// Write writes a packet retrying on ENOBUFS errors.
func (c *packetConn) Write(b []byte) (int, error) {
	var retries uint64
	for {
		n, err := c.Conn.Write(b)
		if n == 0 && err != nil && errors.Is(err, syscall.ENOBUFS) {
			// This is an expected condition on BSD based system. The kernel
			// does not support blocking until buffer space is available.
			// The only way to recover is to retry the call until it
			// succeeds, or drop the packet.
			// Handled in a similar way in gvisor-tap-vsock:
			// https://github.com/containers/gvisor-tap-vsock/issues/367
			time.Sleep(writeRetryDelay)
			retries++
			continue
		}
		if err != nil {
			return 0, err
		}
		if n < len(b) {
			return n, errors.New("incomplete write to unixgram socket")
		}
		if retries > 0 {
			logrus.Debugf("Write completed after %d retries", retries)
		}
		return n, nil
	}
}
