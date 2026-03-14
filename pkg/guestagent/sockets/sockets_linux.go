// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sockets

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// buildInetDiagReqV2 builds the inet_diag_req_v2 bytes (56 bytes total).
func buildInetDiagReqV2(family, proto int) []byte {
	b := make([]byte, 56) // sizeof(inet_diag_req_v2) == 56
	// layout:
	// u8 sdiag_family;
	// u8 sdiag_protocol;
	// u8 idiag_ext;
	// u8 pad;
	// u32 idiag_states;
	// struct inet_diag_sockid { ... }  (48 bytes)
	b[0] = byte(family)
	b[1] = byte(proto)
	b[2] = 0 // ext
	b[3] = 0 // pad
	// idiag_states -> all states
	binary.NativeEndian.PutUint32(b[4:], 0xFFFFFFFF)
	// rest is zero (sockid zero => match all)
	return b
}

func query(conn *netlink.Conn, family, proto int) ([]netlink.Message, error) {
	req := buildInetDiagReqV2(family, proto)
	msg := netlink.Message{
		Header: netlink.Header{
			Type:  unix.SOCK_DIAG_BY_FAMILY,
			Flags: netlink.Request | netlink.Dump,
		},
		Data: req,
	}
	msgs, err := conn.Execute(msg)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func parseMessages(msgs []netlink.Message, proto int) ([]Socket, error) {
	if proto != unix.IPPROTO_TCP && proto != unix.IPPROTO_UDP {
		return nil, fmt.Errorf("unsupported protocol: %d", proto)
	}
	var sockets []Socket
	for _, m := range msgs {
		data := m.Data
		// inet_diag_msg minimum size ~72 bytes (4 + 48 + 20)
		if len(data) < 72 {
			continue
		}
		family := int(data[0])
		state := data[1]
		// data[2] timer, data[3] retrans (ignored here)
		// id begins at offset 4:
		// sport (2B, big-endian), dport (2B, big-endian)
		sport := binary.BigEndian.Uint16(data[4:6])

		src := data[8:24] // 16 bytes

		var localIP net.IP
		if family == unix.AF_INET {
			// first 4 bytes are IPv4 in network order
			localIP = net.IP(src[0:4])
		} else {
			// IPv6
			localIP = net.IP(src)
		}

		// proto name + possible 6 suffix
		pname := "tcp"
		if proto == unix.IPPROTO_UDP {
			pname = "udp"
		}
		if family == unix.AF_INET6 {
			pname += "6"
		}

		newSocket := Socket{
			Kind:  pname,
			IP:    localIP,
			Port:  sport,
			State: state,
		}
		sockets = append(sockets, newSocket)
	}
	return sockets, nil
}

func NewLister() (*Lister, error) {
	conn, err := netlink.Dial(unix.NETLINK_SOCK_DIAG, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open netlink connection: %w", err)
	}
	return &Lister{conn: conn}, nil
}

type Lister struct {
	conn *netlink.Conn
}

func (lister *Lister) List() ([]Socket, error) {
	protos := []int{unix.IPPROTO_TCP, unix.IPPROTO_UDP}
	families := []int{unix.AF_INET, unix.AF_INET6}

	var sockets []Socket
	for _, proto := range protos {
		for _, fam := range families {
			msgs, err := query(lister.conn, fam, proto)
			if err != nil {
				continue
			}
			parsedSockets, err := parseMessages(msgs, proto)
			if err != nil {
				continue
			}
			sockets = append(sockets, parsedSockets...)
		}
	}
	return sockets, nil
}

func (lister *Lister) Close() error {
	return lister.conn.Close()
}
