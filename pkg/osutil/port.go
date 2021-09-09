package osutil

import (
	"github.com/sirupsen/logrus"
	"net"
	"strconv"
	"time"
)

// CheckOrGetFreePort check default port available, not available get new freeport
func CheckOrGetFreePort(port int) int {
	_, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second*3)
	if err != nil {
		return port
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return port
	}
	defer ln.Close()
	// get port
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return port
	}
	newport := tcpAddr.Port
	logrus.Warnf("Check default port %v unavailable, will use free port %v", port, newport)
	return newport
}
