package usernet

import (
	"errors"
	"net"
	"time"
)

type UDPFileConn struct {
	net.Conn
}

func (conn *UDPFileConn) Read(b []byte) (n int, err error) {
	// Check if the connection has been closed
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
			return 0, errors.New("UDPFileConn connection closed")
		}
	}
	return conn.Conn.Read(b)
}
