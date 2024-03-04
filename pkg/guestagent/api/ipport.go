package api

import (
	"net"
	"strconv"
)

func (x *IPPort) HostString() string {
	return net.JoinHostPort(x.GetIp(), strconv.Itoa(int(x.GetPort())))
}
