package api

import (
	"net"
	"strconv"
	"time"
)

var (
	IPv4loopback1 = net.IPv4(127, 0, 0, 1)
)

type IPPort struct {
	IP   net.IP `json:"ip"`
	Port int    `json:"port"`
}

func (x *IPPort) String() string {
	return net.JoinHostPort(x.IP.String(), strconv.Itoa(x.Port))
}

type Info struct {
	// LocalPorts contain 127.0.0.1 and 0.0.0.0.
	// LocalPorts do NOT contain addresses such as 127.0.0.53 and 192.168.5.15.
	//
	// In future, LocalPorts will contain IPv6 addresses (::1 and ::) as well.
	LocalPorts []IPPort `json:"localPorts"`
}

type Event struct {
	Time time.Time `json:"time,omitempty"`
	// The first event contains the full ports as LocalPortsAdded
	LocalPortsAdded   []IPPort `json:"localPortsAdded,omitempty"`
	LocalPortsRemoved []IPPort `json:"localPortsRemoved,omitempty"`
	Errors            []string `json:"errors,omitempty"`
}
