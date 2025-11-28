// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"encoding/binary"
	"net"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// dnsSnooper is a matcher that intercepts DNS response packets and tracks domain-to-IP mappings
// It does not block DNS queries - filtering happens at connection time based on tracked FQDNs.
// The parsing code is just following RFC 1035 vs. using a library. This is less than 100 lines
// of fairly simple RFC following code so on balance worth it.
type dnsSnooper struct {
	tracker *Tracker
}

func (*dnsSnooper) Name() string {
	return "dnsSnooper"
}

func (d *dnsSnooper) Match(_ stack.Hook, pkt *stack.PacketBuffer, _, _ string) (matches, hotdrop bool) {
	// Only inspect UDP packets
	transportHeader := pkt.TransportHeader().Slice()
	if len(transportHeader) < 8 {
		return false, false
	}

	// Check if this is a UDP packet from port 53 (DNS response)
	srcPort := binary.BigEndian.Uint16(transportHeader[0:2])
	if srcPort != 53 {
		return false, false
	}

	// Get the UDP payload (DNS message)
	data := pkt.Data().AsRange().ToSlice()
	if len(data) < 12 {
		return false, false // Too short to be a DNS message
	}

	// Parse DNS header
	// Byte 2-3: Flags - check if this is a response (QR bit = 1)
	flags := binary.BigEndian.Uint16(data[2:4])
	isResponse := (flags & 0x8000) != 0
	if !isResponse {
		return false, false
	}

	// Get question count and answer count
	qdCount := binary.BigEndian.Uint16(data[4:6])
	anCount := binary.BigEndian.Uint16(data[6:8])

	if anCount == 0 {
		return false, false // No answers
	}

	// Parse DNS message
	domain, ips, ttl := parseDNSResponse(data, qdCount, anCount)
	if domain != "" && len(ips) > 0 {
		// Track DNS response for future FQDN-based connection filtering
		// We don't block DNS queries themselves - only actual connections to unauthorized IPs
		// Use minimum TTL of 60 seconds to handle cases where DNS TTL is 0 or very short
		trackTTL := ttl
		if trackTTL < 60 {
			trackTTL = 60
		}
		d.tracker.AddRecord(domain, ips, time.Duration(trackTTL)*time.Second)
	}

	// Always allow DNS packets - filtering happens at connection time
	return false, false
}

// parseDNSResponse extracts domain, IPs, and TTL from a DNS response packet.
func parseDNSResponse(data []byte, qdCount, anCount uint16) (domain string, ips []net.IP, ttl uint32) {
	offset := 12 // Start after DNS header

	// Skip questions to get to answers
	for i := uint16(0); i < qdCount && offset < len(data); i++ {
		// Parse the domain name from the question
		if domain == "" {
			domain, offset = parseDNSName(data, offset)
		} else {
			_, offset = parseDNSName(data, offset)
		}
		if offset+4 > len(data) {
			return "", nil, 0
		}
		offset += 4 // Skip QTYPE and QCLASS
	}

	// Parse answers
	var minTTL uint32
	firstTTL := true

	for i := uint16(0); i < anCount && offset < len(data); i++ {
		// Parse name (might be compressed)
		_, offset = parseDNSName(data, offset)
		if offset+10 > len(data) {
			break
		}

		rrType := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2 // TYPE
		offset += 2 // CLASS

		ttl := binary.BigEndian.Uint32(data[offset : offset+4])
		if firstTTL || ttl < minTTL {
			minTTL = ttl
			firstTTL = false
		}
		offset += 4 // TTL

		rdLength := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2 // RDLENGTH

		if offset+int(rdLength) > len(data) {
			break
		}

		// Extract IP addresses from A (1) and AAAA (28) records
		if rrType == 1 && rdLength == 4 {
			// A record (IPv4)
			ip := net.IP(data[offset : offset+4])
			ips = append(ips, ip)
		} else if rrType == 28 && rdLength == 16 {
			// AAAA record (IPv6)
			ip := net.IP(data[offset : offset+16])
			ips = append(ips, ip)
		}

		offset += int(rdLength)
	}

	return domain, ips, minTTL
}

// parseDNSName parses a DNS name and returns the name and new offset.
func parseDNSName(data []byte, offset int) (name string, newOffset int) {
	jumped := false
	jumpOffset := 0
	maxJumps := 5
	jumps := 0

	for offset < len(data) {
		length := int(data[offset])

		// Check for compression (pointer)
		if length&0xC0 == 0xC0 {
			if offset+1 >= len(data) {
				break
			}
			pointer := int(binary.BigEndian.Uint16(data[offset:offset+2]) & 0x3FFF)
			if !jumped {
				jumpOffset = offset + 2
			}
			offset = pointer
			jumped = true
			jumps++
			if jumps > maxJumps {
				break
			}
			continue
		}

		if length == 0 {
			offset++
			break
		}

		offset++
		if offset+length > len(data) {
			break
		}

		if name != "" {
			name += "."
		}
		name += string(data[offset : offset+length])
		offset += length
	}

	if jumped {
		return name, jumpOffset
	}
	return name, offset
}
