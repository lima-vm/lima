// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sockets

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func NewLister() (*Lister, error) {
	return &Lister{}, nil
}

type Lister struct{}

func (lister *Lister) List() ([]Socket, error) {
	var sockets []Socket
	for _, proto := range []string{"tcp", "udp"} {
		// -an: all sockets, numeric; -p: protocol filter
		// Without -f, netstat shows both inet and inet6.
		out, err := exec.Command("netstat", "-an", "-p", proto).Output()
		if err != nil {
			continue
		}
		parsed := parseNetstatOutput(string(out), proto)
		sockets = append(sockets, parsed...)
	}
	return sockets, nil
}

func (lister *Lister) Close() error {
	return nil
}

// parseNetstatOutput parses macOS `netstat -an -p {tcp,udp}` output.
// The protocol field (tcp4/tcp6/udp4/udp6) determines the address family.
//
// Example lines:
//
//	tcp4       0      0  *.22                   *.*                    LISTEN
//	tcp4       0      0  127.0.0.1.8080         *.*                    LISTEN
//	tcp6       0      0  *.22                   *.*                    LISTEN
//	udp4       0      0  *.5353                 *.*
func parseNetstatOutput(output, proto string) []Socket {
	var sockets []Socket
	isTCP := proto == "tcp"
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		// Minimum fields: proto recvq sendq local foreign [state]
		if len(fields) < 5 {
			continue
		}

		protoField := fields[0]
		if !strings.HasPrefix(protoField, proto) {
			continue
		}

		// Determine address family from protocol field suffix
		isIPv6 := strings.HasSuffix(protoField, "6")

		if isTCP {
			// TCP requires a 6th field (state) to be LISTEN
			if len(fields) < 6 || fields[5] != "LISTEN" {
				continue
			}
		} else {
			// UDP: unconnected sockets have foreign address *.*
			if fields[4] != "*.*" {
				continue
			}
		}

		ip, port, err := parseNetstatAddr(fields[3], isIPv6)
		if err != nil {
			continue
		}

		kind := proto
		if isIPv6 {
			kind += "6"
		}

		var state State
		if isTCP {
			state = TCPListen
		} else {
			state = UDPUnconnected
		}

		sockets = append(sockets, Socket{
			Kind:  kind,
			IP:    ip,
			Port:  port,
			State: state,
		})
	}
	return sockets
}

// parseNetstatAddr parses a macOS netstat local address.
// The format is IP.Port where the last dot separates the port.
// Examples: "127.0.0.1.8080", "*.22", "::1.8080", "fe80::1%lo0.22".
func parseNetstatAddr(addr string, isIPv6 bool) (net.IP, uint16, error) {
	lastDot := strings.LastIndex(addr, ".")
	if lastDot < 0 {
		return nil, 0, fmt.Errorf("no dot in address %q", addr)
	}

	ipStr := addr[:lastDot]
	port, err := strconv.ParseUint(addr[lastDot+1:], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port in %q: %w", addr, err)
	}

	var ip net.IP
	switch {
	case ipStr == "*":
		if isIPv6 {
			ip = net.IPv6zero
		} else {
			ip = net.IPv4zero
		}
	default:
		// Strip zone ID (e.g., "%lo0") before parsing
		if i := strings.IndexByte(ipStr, '%'); i >= 0 {
			ipStr = ipStr[:i]
		}
		ip = net.ParseIP(ipStr)
		if ip == nil {
			return nil, 0, fmt.Errorf("invalid IP in %q", addr)
		}
	}

	return ip, uint16(port), nil
}
