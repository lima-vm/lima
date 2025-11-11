// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestParseDNSResponse(t *testing.T) {
	response := buildDNSResponse("example.com", net.ParseIP("93.184.216.34"), 300)
	domain, ips, ttl := parseDNSResponse(response, 1, 1)

	assert.Equal(t, domain, "example.com")
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0].String(), "93.184.216.34")
	assert.Equal(t, ttl, uint32(300))
}

func TestParseDNSResponse_IPv6(t *testing.T) {
	response := buildDNSResponse("ipv6.example.com", net.ParseIP("2001:db8::1"), 600)
	domain, ips, ttl := parseDNSResponse(response, 1, 1)

	assert.Equal(t, domain, "ipv6.example.com")
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0].String(), "2001:db8::1")
	assert.Equal(t, ttl, uint32(600))
}

func TestParseDNSResponse_MultipleIPs(t *testing.T) {
	response := buildDNSResponseMulti("multi.example.com", []net.IP{
		net.ParseIP("192.0.2.1"),
		net.ParseIP("192.0.2.2"),
		net.ParseIP("192.0.2.3"),
	}, 120)

	domain, ips, ttl := parseDNSResponse(response, 1, 3)

	assert.Equal(t, domain, "multi.example.com")
	assert.Equal(t, len(ips), 3)

	expectedIPs := []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"}
	for i, ip := range ips {
		assert.Equal(t, ip.String(), expectedIPs[i])
	}

	assert.Equal(t, ttl, uint32(120))
}

func TestDNSSnooperIntegration(t *testing.T) {
	tracker := NewTracker()
	pol := &Policy{
		Version: "1.0",
		Rules: []PolicyRule{
			{Name: "allow-all", Action: "allow", Priority: 100},
		},
	}

	table, err := BuildFilterTable(pol, tracker, "192.168.127.0/24", "192.168.127.1", false)
	assert.NilError(t, err)
	assert.Assert(t, len(table.Rules) >= 2, "Expected at least 2 rules (snooper + policy)")

	response := buildDNSResponse("github.com", net.ParseIP("140.82.121.4"), 60)
	domain, ips, ttl := parseDNSResponse(response, 1, 1)

	assert.Assert(t, domain != "" && len(ips) > 0, "Failed to parse DNS response")

	tracker.AddRecord(domain, ips, time.Duration(ttl)*time.Second)

	trackedIPs := tracker.GetIPs("github.com")
	assert.Equal(t, len(trackedIPs), 1)
	assert.Equal(t, trackedIPs[0].String(), "140.82.121.4")
}

// Helper functions to build DNS response packets

func buildDNSResponse(domain string, ip net.IP, ttl uint32) []byte {
	return buildDNSResponseMulti(domain, []net.IP{ip}, ttl)
}

func buildDNSResponseMulti(domain string, ips []net.IP, ttl uint32) []byte {
	buf := make([]byte, 0, 512)

	// DNS Header (12 bytes)
	buf = append(buf, 0x00, 0x01) // Transaction ID
	buf = append(buf, 0x81, 0x80) // Flags: response, no error
	buf = append(buf, 0x00, 0x01) // Questions: 1
	anCount := uint16(len(ips))
	buf = binary.BigEndian.AppendUint16(buf, anCount) // Answers
	buf = append(buf, 0x00, 0x00)                     // Authority RRs: 0
	buf = append(buf, 0x00, 0x00)                     // Additional RRs: 0

	// Question section
	buf = appendDNSName(buf, domain)
	if ips[0].To4() != nil {
		buf = append(buf, 0x00, 0x01) // Type A
	} else {
		buf = append(buf, 0x00, 0x1c) // Type AAAA
	}
	buf = append(buf, 0x00, 0x01) // Class IN

	// Answer section(s)
	for _, ip := range ips {
		buf = appendDNSName(buf, domain)
		if ip.To4() != nil {
			buf = append(buf, 0x00, 0x01) // Type A
			buf = append(buf, 0x00, 0x01) // Class IN
			buf = binary.BigEndian.AppendUint32(buf, ttl)
			buf = append(buf, 0x00, 0x04) // RDLENGTH: 4
			buf = append(buf, ip.To4()...)
		} else {
			buf = append(buf, 0x00, 0x1c) // Type AAAA
			buf = append(buf, 0x00, 0x01) // Class IN
			buf = binary.BigEndian.AppendUint32(buf, ttl)
			buf = append(buf, 0x00, 0x10) // RDLENGTH: 16
			buf = append(buf, ip.To16()...)
		}
	}

	return buf
}

func appendDNSName(buf []byte, domain string) []byte {
	labels := splitDomain(domain)
	for _, label := range labels {
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0x00) // Null terminator
	return buf
}

func splitDomain(domain string) []string {
	var labels []string
	current := ""
	for _, ch := range domain {
		if ch == '.' {
			if current != "" {
				labels = append(labels, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		labels = append(labels, current)
	}
	return labels
}
