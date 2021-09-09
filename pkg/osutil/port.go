package osutil

import (
	"net"
)

// CheckOrGetFreePort check default port available, not available get new freeport
func CheckOrGetFreePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer ln.Close()
	// get port
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	newport := tcpAddr.Port
	return newport
}
