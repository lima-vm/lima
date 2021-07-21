// Package getaddrinfoworkaround launches a mDNS for the host's hostname when the hostname is not present in
// `/etc/hosts` on the host filesystem.
//
// Without this weird workaround, mount may take 25 secs.
// ```
// [2021/07/21 18:47:12.871531,  3] ../../lib/util/util_net.c:257(interpret_string_addr_internal)
//   interpret_string_addr_internal: getaddrinfo failed for name suda-mbp.local (flags 1026) [nodename nor servname provided, or not known]
// [2021/07/21 18:47:12.871626,  3] ../../source3/lib/util_sock.c:1026(get_mydnsfullname)
//   get_mydnsfullname: getaddrinfo failed for name suda-mbp.local [Unknown error]
// ```
//
// https://github.com/lima-vm/lima/pull/118#issuecomment-887276121
// https://lists.samba.org/archive/samba/2017-September/210808.html
package getaddrinfoworkaround

import (
	"net"
	"os"
	"strings"

	"github.com/pion/mdns"
	"golang.org/x/net/ipv4"
)

func Needed() (bool, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return false, err
	}
	hosts, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return false, err
	}

	// FIXME: not robust
	// TODO: scan the lines
	hostsHasHostname := strings.Contains(string(hosts), hostname)
	needed := !hostsHasHostname
	return needed, nil
}

func New() *Workaround {
	w := &Workaround{}
	return w
}

type Workaround struct {
	c *mdns.Conn
}

func (w *Workaround) Start() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	config := &mdns.Config{
		LocalNames: []string{hostname},
	}
	udpAddr, err := net.ResolveUDPAddr("udp", mdns.DefaultAddress)
	if err != nil {
		return err
	}
	listener, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return err
	}
	packetConn := ipv4.NewPacketConn(listener)
	w.c, err = mdns.Server(packetConn, config)
	return err
}

func (w *Workaround) Close() error {
	if w.c != nil {
		return w.c.Close()
	}
	return nil
}
