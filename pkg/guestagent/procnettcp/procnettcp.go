// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package procnettcp

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"golang.org/x/sys/cpu"
)

type Kind = string

const (
	TCP  Kind = "tcp"
	TCP6 Kind = "tcp6"
	UDP  Kind = "udp"
	UDP6 Kind = "udp6"
	// TODO: "udplite", "udplite6".
)

type State = int

const (
	TCPEstablished State = 0x1
	TCPListen      State = 0xA
	UDPEstablished State = 0x7
)

type Entry struct {
	Kind  Kind   `json:"kind"`
	IP    net.IP `json:"ip"`
	Port  uint16 `json:"port"`
	State State  `json:"state"`
}

func Parse(r io.Reader, kind Kind) ([]Entry, error) {
	return ParseWithEndian(r, kind, cpu.IsBigEndian)
}

func ParseWithEndian(r io.Reader, kind Kind, isBE bool) ([]Entry, error) {
	switch kind {
	case TCP, TCP6, UDP, UDP6:
	default:
		return nil, fmt.Errorf("unexpected kind %q", kind)
	}

	var entries []Entry
	sc := bufio.NewScanner(r)

	// As of kernel 5.11, ["local_address"] = 1
	fieldNames := make(map[string]int)
	for i := 0; sc.Scan(); i++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch i {
		case 0:
			for j := range fields {
				fieldNames[fields[j]] = j
			}
			if _, ok := fieldNames["local_address"]; !ok {
				return nil, errors.New("field \"local_address\" not found")
			}
			if _, ok := fieldNames["st"]; !ok {
				return nil, errors.New("field \"st\" not found")
			}

		default:
			// localAddress is like "0100007F:053A"
			localAddress := fields[fieldNames["local_address"]]
			ip, port, err := ParseAddressWithEndian(localAddress, isBE)
			if err != nil {
				return entries, err
			}

			stStr := fields[fieldNames["st"]]
			st, err := strconv.ParseUint(stStr, 16, 8)
			if err != nil {
				return entries, err
			}

			ent := Entry{
				Kind:  kind,
				IP:    ip,
				Port:  port,
				State: int(st),
			}
			entries = append(entries, ent)
		}
	}

	if err := sc.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}

// ParseAddress parses a string.
//
// Little endian hosts:
// "0100007F:0050"                         (127.0.0.1:80)
// "000080FE00000000FF57A6705DC771FE:0050" ([fe80::70a6:57ff:fe71:c75d]:80)
// "00000000000000000000000000000000:0050" (0.0.0.0:80)
//
// Big endian hosts:
// "7F000001:0050"                         (127.0.0.1:80)
// "FE8000000000000070A657FFFE71C75D:0050" ([fe80::70a6:57ff:fe71:c75d]:80)
// "00000000000000000000000000000000:0050" (0.0.0.0:80)
//
// See https://serverfault.com/questions/592574/why-does-proc-net-tcp6-represents-1-as-1000
func ParseAddress(s string) (net.IP, uint16, error) {
	return ParseAddressWithEndian(s, cpu.IsBigEndian)
}

func ParseAddressWithEndian(s string, isBE bool) (net.IP, uint16, error) {
	split := strings.SplitN(s, ":", 2)
	if len(split) != 2 {
		return nil, 0, fmt.Errorf("unparsable address %q", s)
	}
	switch l := len(split[0]); l {
	case 8, 32:
	default:
		return nil, 0, fmt.Errorf("unparsable address %q, expected length of %q to be 8 or 32, got %d",
			s, split[0], l)
	}

	ipBytes := make([]byte, len(split[0])/2) // 4 bytes (8 chars) or 16 bytes (32 chars)
	for i := range len(split[0]) / 8 {
		quartet := split[0][8*i : 8*(i+1)]
		quartetB, err := hex.DecodeString(quartet) // surprisingly little endian, per 4 bytes, on little endian hosts
		if err != nil {
			return nil, 0, fmt.Errorf("unparsable address %q: unparsable quartet %q: %w", s, quartet, err)
		}
		if isBE {
			for j := range quartetB {
				ipBytes[4*i+j] = quartetB[j]
			}
		} else {
			for j := range quartetB {
				ipBytes[4*i+len(quartetB)-1-j] = quartetB[j]
			}
		}
	}
	ip := net.IP(ipBytes)

	port64, err := strconv.ParseUint(split[1], 16, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("unparsable address %q: unparsable port %q", s, split[1])
	}
	port := uint16(port64)

	return ip, port, nil
}
