//go:build darwin && !no_vz

package vz

import (
	"encoding/binary"
	"fmt"
	"net"
	"path/filepath"
	"testing"
)

const vmnetMaxPacketSize = 1514
const packetsCount = 1000

func TestDialQemu(t *testing.T) {
	listener, err := listenUnix(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Logf("Listening at %q", listener.Addr())

	errc := make(chan error, 2)

	// Start the fake vmnet server.
	go func() {
		t.Log("Fake vmnet started")
		errc <- serveOneClient(listener)
		t.Log("Fake vmnet finished")
	}()

	// Connect to the fake vmnet server.
	client, err := DialQemu(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Connected to fake vment server")

	dgramConn, err := net.FileConn(client)
	if err != nil {
		t.Fatal(err)
	}

	vzConn := packetConn{Conn: dgramConn}
	defer vzConn.Close()

	go func() {
		t.Log("Sender started")
		buf := make([]byte, vmnetMaxPacketSize)
		for i := 0; i < vmnetMaxPacketSize; i++ {
			buf[i] = 0x55
		}

		// data packet format:
		//     0-4		packet number
		//     4-1514	0x55 ...
		for i := 0; i < packetsCount; i++ {
			binary.BigEndian.PutUint32(buf, uint32(i))
			if _, err := vzConn.Write(buf); err != nil {
				errc <- err
				return
			}
		}
		t.Logf("Sent %d data packets", packetsCount)

		// quit packet format:
		//     0-4:     "quit"
		copy(buf[:4], []byte("quit"))
		if _, err := vzConn.Write(buf[:4]); err != nil {
			errc <- err
			return
		}

		errc <- nil
		t.Log("Sender finished")
	}()

	// Read and verify packets to the server.

	buf := make([]byte, vmnetMaxPacketSize)

	t.Logf("Receiving and verifying data packets...")
	for i := 0; i < packetsCount; i++ {
		n, err := vzConn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if n < vmnetMaxPacketSize {
			t.Fatalf("Expected %d bytes, got %d", vmnetMaxPacketSize, n)
		}

		number := binary.BigEndian.Uint32(buf[:4])
		if number != uint32(i) {
			t.Fatalf("Expected packet %d, got packet %d", i, number)
		}

		for j := 4; j < vmnetMaxPacketSize; j++ {
			if buf[j] != 0x55 {
				t.Fatalf("Expected byte 0x55 at offset %d, got 0x%02x", j, buf[j])
			}
		}
	}
	t.Logf("Recived and verified %d data packets", packetsCount)

	for i := 0; i < 2; i++ {
		err := <-errc
		if err != nil {
			t.Fatal(err)
		}
	}
}

// serveOneClient accepts one client and echo back received packets until a
// "quit" packet is sent.
func serveOneClient(listener *net.UnixListener) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	qemuConn := qemuPacketConn{Conn: conn}
	defer qemuConn.Close()

	buf := make([]byte, vmnetMaxPacketSize)
	for {
		nr, err := qemuConn.Read(buf)
		if err != nil {
			return err
		}
		if string(buf[:4]) == "quit" {
			return nil
		}
		nw, err := qemuConn.Write(buf[:nr])
		if err != nil {
			return err
		}
		if nw != nr {
			return fmt.Errorf("incomplete write: expected: %d, wrote: %d", nr, nw)
		}
	}
}

// listenUnix creates and listen to unix socket under dir
func listenUnix(dir string) (*net.UnixListener, error) {
	sock := filepath.Join(dir, "sock")
	addr, err := net.ResolveUnixAddr("unix", sock)
	if err != nil {
		return nil, err
	}
	return net.ListenUnix("unix", addr)
}
