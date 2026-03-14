// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sockets

import (
	"net"
)

type Kind = string

const (
	TCP  Kind = "tcp"
	TCP6 Kind = "tcp6"
	UDP  Kind = "udp"
	UDP6 Kind = "udp6"
	// TODO: "udplite", "udplite6".
)

type State = byte

const (
	TCPEstablished State = 0x1
	TCPListen      State = 0xA
	UDPUnconnected State = 0x7
)

type Socket struct {
	Kind  Kind   `json:"kind"`
	IP    net.IP `json:"ip"`
	Port  uint16 `json:"port"`
	State State  `json:"state"`
}
