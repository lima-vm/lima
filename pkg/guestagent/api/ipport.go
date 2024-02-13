package api

import (
	"net"
	"strconv"
)

func (x *IPPort) HostString() string {
	return net.JoinHostPort(x.Ip, strconv.Itoa(int(x.Port)))
}
