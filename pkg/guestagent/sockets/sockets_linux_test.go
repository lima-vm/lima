// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sockets

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

// helper to construct a minimal inet_diag_msg-like payload.
// Layout assumptions mirror parseMessages:
//
//	byte 0: family
//	byte 1: state
//	bytes 4-6: sport (big-endian)
//	bytes 8-24: source address (16 bytes, IPv4 in first 4 if AF_INET)
func diagMsg(family int, state byte, port uint16, ip net.IP) netlink.Message {
	data := make([]byte, 72) // minimum size accepted
	data[0] = byte(family)
	data[1] = state
	// sport
	binary.BigEndian.PutUint16(data[4:6], port)
	src := data[8:24]
	if family == unix.AF_INET {
		ip4 := ip.To4()
		copy(src[:4], ip4)
	} else {
		ip16 := ip.To16()
		copy(src, ip16)
	}
	return netlink.Message{Data: data}
}

func TestBuildInetDiagReqV2(t *testing.T) {
	req := buildInetDiagReqV2(unix.AF_INET6, unix.IPPROTO_TCP)
	assert.Equal(t, len(req), 56, "unexpected request length")
	if req[0] != byte(unix.AF_INET6) {
		t.Errorf("family byte = %d want %d", req[0], unix.AF_INET6)
	}
	if req[1] != byte(unix.IPPROTO_TCP) {
		t.Errorf("proto byte = %d want %d", req[1], unix.IPPROTO_TCP)
	}
	states := binary.LittleEndian.Uint32(req[4:8])
	if states != 0xFFFFFFFF {
		t.Errorf("idiag_states = 0x%08X want 0xFFFFFFFF", states)
	}
}

func TestParseMessages_TCPv4(t *testing.T) {
	const port = 8080
	ip := net.ParseIP("127.0.0.1")
	msg := diagMsg(unix.AF_INET, TCPListen, port, ip)
	socks, err := parseMessages([]netlink.Message{msg}, unix.IPPROTO_TCP)
	assert.NilError(t, err, "parseMessages error")
	assert.Equal(t, len(socks), 1, "unexpected number of sockets")
	s := socks[0]
	if s.Kind != "tcp" {
		t.Errorf("Kind = %q want tcp", s.Kind)
	}
	if !s.IP.Equal(ip) {
		t.Errorf("IP = %v want %v", s.IP, ip)
	}
	if s.Port != port {
		t.Errorf("Port = %d want %d", s.Port, port)
	}
	if s.State != TCPListen {
		t.Errorf("State = 0x%X want 0x%X", s.State, TCPListen)
	}
}

func TestParseMessages_UDPv6(t *testing.T) {
	const port = 5353
	ip := net.ParseIP("2001:db8::1")
	msg := diagMsg(unix.AF_INET6, UDPUnconnected, port, ip)
	socks, err := parseMessages([]netlink.Message{msg}, unix.IPPROTO_UDP)
	assert.NilError(t, err, "parseMessages error")
	assert.Equal(t, len(socks), 1, "unexpected number of sockets")
	s := socks[0]
	if s.Kind != "udp6" {
		t.Errorf("Kind = %q want udp6", s.Kind)
	}
	if !s.IP.Equal(ip) {
		t.Errorf("IP = %v want %v", s.IP, ip)
	}
	if s.Port != port {
		t.Errorf("Port = %d want %d", s.Port, port)
	}
	if s.State != UDPUnconnected {
		t.Errorf("State = 0x%X want 0x%X", s.State, UDPUnconnected)
	}
}

func TestParseMessages_ShortDataSkipped(t *testing.T) {
	short := netlink.Message{Data: make([]byte, 10)} // < 72 => ignored
	socks, err := parseMessages([]netlink.Message{short}, unix.IPPROTO_TCP)
	assert.NilError(t, err, "parseMessages error")
	assert.Equal(t, len(socks), 0, "unexpected number of sockets")
}

func TestListS_Integration(t *testing.T) {
	_, err := List()
	if err != nil {
		t.Skipf("skipping: cannot query netlink inet_diag (%v)", err)
	}
	// No assertions: presence of error-free call is sufficient.
}
