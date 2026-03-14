// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"runtime"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/miekg/dns"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

var dnsResult *dns.Msg

func TestNewHandler(t *testing.T) {
	t.Run("with upstream servers", func(t *testing.T) {
		upstreamServers := []string{"8.8.4.4", "1.1.1.1", "9.9.9.9"}
		opts := HandlerOptions{
			IPv6:            true,
			UpstreamServers: upstreamServers,
			StaticHosts: map[string]string{
				"test.local":  "192.168.1.1",
				"alias.local": "test.local",
			},
		}
		h, err := NewHandler(opts)
		assert.NilError(t, err)
		assert.Assert(t, h != nil)

		handler := h.(*Handler)
		assert.Equal(t, handler.ipv6, true)
		assert.Equal(t, len(handler.clients), 2)
		assert.DeepEqual(t, handler.clientConfig.Servers, upstreamServers)
		assert.Equal(t, handler.hostToIP["test.local."].String(), "192.168.1.1")
		assert.Equal(t, handler.cnameToHost["alias.local."], "test.local.")
	})

	t.Run("without upstream servers on non-Windows", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping on Windows")
		}
		opts := HandlerOptions{
			IPv6:        false,
			StaticHosts: map[string]string{},
		}
		h, err := NewHandler(opts)
		assert.NilError(t, err)
		assert.Assert(t, h != nil)

		handler := h.(*Handler)
		assert.Equal(t, handler.ipv6, false)
		assert.Assert(t, handler.clientConfig != nil)
	})

	t.Run("without upstream servers on Windows", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Skipping on non-Windows")
		}
		opts := HandlerOptions{
			IPv6:        false,
			StaticHosts: map[string]string{},
		}
		h, err := NewHandler(opts)
		assert.NilError(t, err)
		assert.Assert(t, h != nil)

		handler := h.(*Handler)
		assert.Equal(t, handler.ipv6, false)
		assert.Assert(t, handler.clientConfig != nil)
		// Should use default fallback IPs on Windows
		assert.Assert(t, len(handler.clientConfig.Servers) > 0)
	})

	t.Run("with invalid upstream servers fallback", func(t *testing.T) {
		opts := HandlerOptions{
			IPv6:            true,
			UpstreamServers: []string{}, // empty should trigger default behavior
			StaticHosts:     map[string]string{},
		}
		h, err := NewHandler(opts)
		assert.NilError(t, err)
		assert.Assert(t, h != nil)
	})

	t.Run("with static hosts IP and CNAME", func(t *testing.T) {
		opts := HandlerOptions{
			IPv6:            true,
			UpstreamServers: []string{"8.8.8.8"},
			StaticHosts: map[string]string{
				"host1.local": "10.0.0.1",
				"host2.local": "10.0.0.2",
				"cname1":      "host1.local",
				"cname2":      "cname1",
			},
		}
		h, err := NewHandler(opts)
		assert.NilError(t, err)
		assert.Assert(t, h != nil)

		handler := h.(*Handler)
		assert.Equal(t, handler.hostToIP["host1.local."].String(), "10.0.0.1")
		assert.Equal(t, handler.hostToIP["host2.local."].String(), "10.0.0.2")
		assert.Equal(t, handler.cnameToHost["cname1."], "host1.local.")
		assert.Equal(t, handler.cnameToHost["cname2."], "cname1.")
	})

	t.Run("with truncate option", func(t *testing.T) {
		opts := HandlerOptions{
			IPv6:            false,
			UpstreamServers: []string{"1.1.1.1"},
			TruncateReply:   true,
			StaticHosts:     map[string]string{},
		}
		h, err := NewHandler(opts)
		assert.NilError(t, err)
		assert.Assert(t, h != nil)

		handler := h.(*Handler)
		assert.Equal(t, handler.truncate, true)
	})
}

func TestDNSRecords(t *testing.T) {
	if runtime.GOOS == "windows" {
		// "On Windows, the resolver always uses C library functions, such as GetAddrInfo and DnsQuery."
		t.Skip()
	}

	srv, err := mockdns.NewServerWithLogger(map[string]mockdns.Zone{
		"onerecord.com.": {
			TXT: []string{"My txt record"},
		},
		"multistringrecord.com.": {
			TXT: []string{"123456789012345678901234567890123456789012345678901234567890" +
				"123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890" +
				"123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890" +
				"12345678901234567890123456789012345678901234567890"},
		},
		"multiplerecords.com.": {
			TXT: []string{"record 1", "record 2"},
		},
	}, log.New(io.Discard, "mockdns server: ", log.LstdFlags), false)
	assert.NilError(t, err)
	defer srv.Close()

	srv.PatchNet(net.DefaultResolver)
	defer mockdns.UnpatchNet(net.DefaultResolver)
	w := new(TestResponseWriter)
	options := HandlerOptions{
		IPv6: true,
		StaticHosts: map[string]string{
			"MY.DOMAIN.COM":      "192.168.0.23",
			"host.lima.internal": "10.10.0.34",
			"my.host":            "host.lima.internal",
			"default":            "my.domain.com",
			"cycle1.example.com": "cycle2.example.com",
			"cycle2.example.com": "cycle1.example.com",
			"self.example.com":   "self.example.com",
		},
	}

	h, err := NewHandler(options)
	assert.NilError(t, err)

	regexMatch := func(value string, pattern string) cmp.Comparison {
		return func() cmp.Result {
			re := regexp.MustCompile(pattern)
			if re.MatchString(value) {
				return cmp.ResultSuccess
			}
			return cmp.ResultFailure(
				fmt.Sprintf("%q did not match pattern %q", value, pattern))
		}
	}
	t.Run("test TXT records", func(t *testing.T) {
		tests := []struct {
			testDomain        string
			expectedTXTRecord string
		}{
			{testDomain: "onerecord.com", expectedTXTRecord: `onerecord.com.\s+5\s+IN\s+TXT\s+"My txt record"`},
			{testDomain: "multistringrecord.com", expectedTXTRecord: `multistringrecord.com.\s+5\s+IN\s+TXT\s+"123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345" "67890123456789012345678901234567890"`},
			{testDomain: "multiplerecords.com", expectedTXTRecord: `multiplerecords.com.\s+5\s+IN\s+TXT\s+"record 1"\nmultiplerecords.com.\s*5\s*IN\s*TXT\s*"record 2"`},
		}

		for _, tc := range tests {
			req := new(dns.Msg)
			req.SetQuestion(dns.Fqdn(tc.testDomain), dns.TypeTXT)
			h.ServeDNS(w, req)
			assert.Assert(t, regexMatch(dnsResult.String(), tc.expectedTXTRecord))
		}
	})

	t.Run("test A records", func(t *testing.T) {
		tests := []struct {
			testDomain      string
			expectedARecord string
		}{
			{testDomain: "my.domain.com", expectedARecord: `my.domain.com.\s+5\s+IN\s+A\s+192.168.0.23`},
			{testDomain: "host.lima.internal", expectedARecord: `host.lima.internal.\s+5\s+IN\s+A\s+10.10.0.34`},
		}

		for _, tc := range tests {
			req := new(dns.Msg)
			req.SetQuestion(dns.Fqdn(tc.testDomain), dns.TypeA)
			h.ServeDNS(w, req)
			assert.Assert(t, regexMatch(dnsResult.String(), tc.expectedARecord))
		}
	})

	t.Run("test CNAME records", func(t *testing.T) {
		tests := []struct {
			testDomain    string
			expectedCNAME string
		}{
			{testDomain: "my.host", expectedCNAME: `my.host.\s+5\s+IN\s+CNAME\s+host.lima.internal.`},
			{testDomain: "default", expectedCNAME: `default.\s+5\s+IN\s+CNAME\s+my.domain.com.`},
		}

		for _, tc := range tests {
			req := new(dns.Msg)
			req.SetQuestion(dns.Fqdn(tc.testDomain), dns.TypeCNAME)
			h.ServeDNS(w, req)
			assert.Assert(t, regexMatch(dnsResult.String(), tc.expectedCNAME))
		}
	})

	t.Run("test cyclic CNAME records", func(t *testing.T) {
		tests := []struct {
			testDomain    string
			expectedCNAME string
		}{
			{testDomain: "cycle1.example.com", expectedCNAME: `cycle1.example.com.`},
			{testDomain: "self.example.com", expectedCNAME: `self.example.com.`},
		}

		for _, tc := range tests {
			req := new(dns.Msg)
			req.SetQuestion(dns.Fqdn(tc.testDomain), dns.TypeCNAME)
			h.ServeDNS(w, req)
			assert.Assert(t, regexMatch(dnsResult.String(), tc.expectedCNAME))
		}
	})
}

type TestResponseWriter struct{}

// LocalAddr returns the net.Addr of the server
func (r TestResponseWriter) LocalAddr() net.Addr {
	return new(TestAddr)
}

// RemoteAddr returns the net.Addr of the client that sent the current request.
func (r TestResponseWriter) RemoteAddr() net.Addr {
	return new(TestAddr)
}

// Network returns the value of the Net field of the Server (e.g., "tcp", "tcp-tls").
func (r TestResponseWriter) Network() string {
	return ""
}

// WriteMsg writes a reply back to the client.
func (r TestResponseWriter) WriteMsg(newMsg *dns.Msg) error {
	dnsResult = newMsg
	return nil
}

// Write writes a raw buffer back to the client.
func (r TestResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

// Close closes the connection.
func (r TestResponseWriter) Close() error {
	return nil
}

// TsigStatus returns the status of the Tsig.
func (r TestResponseWriter) TsigStatus() error {
	return nil
}

// TsigTimersOnly sets the tsig timers only boolean.
func (r TestResponseWriter) TsigTimersOnly(bool) {
}

// Hijack lets the caller take over the connection.
// After a call to Hijack(), the DNS package will not do anything with the connection.
func (r TestResponseWriter) Hijack() {
}

type TestAddr struct{}

func (r TestAddr) Network() string {
	return ""
}

func (r TestAddr) String() string {
	return ""
}
