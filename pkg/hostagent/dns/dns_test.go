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

var (
	dnsResult *dns.Msg
)

func TestTXTRecords(t *testing.T) {
	testDomains := make([]string, 3)
	testDomains[0] = "onerecord.com"
	testDomains[1] = "multistringrecord.com"
	testDomains[2] = "multiplerecords.com"

	expectedResults := make([]string, 3)
	expectedResults[0] = `onerecord.com.\s+5\s+IN\s+TXT\s+"My txt record"`
	expectedResults[1] = `multistringrecord.com.\s+5\s+IN\s+TXT\s+"123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345" "67890123456789012345678901234567890"`
	expectedResults[2] = `multiplerecords.com.\s+5\s+IN\s+TXT\s+"record 1"\nmultiplerecords.com.\s*5\s*IN\s*TXT\s*"record 2"`

	srv, _ := mockdns.NewServerWithLogger(map[string]mockdns.Zone{
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
	defer srv.Close()

	if runtime.GOOS == "windows" {
		// "On Windows, the resolver always uses C library functions, such as GetAddrInfo and DnsQuery."
		t.Skip()
	}
	srv.PatchNet(net.DefaultResolver)
	defer mockdns.UnpatchNet(net.DefaultResolver)

	t.Run("test TXT records", func(t *testing.T) {
		w := new(TestResponseWriter)
		h, err := newHandler(true, map[string]string{
			"MY.Host": "host.lima.internal",
		})
		if err == nil {
			for i := 0; i < len(testDomains); i++ {
				req := new(dns.Msg)
				req.SetQuestion(dns.Fqdn(testDomains[i]), dns.TypeTXT)
				h.ServeDNS(w, req)
				regex_match := func(value string, pattern string) cmp.Comparison {
					return func() cmp.Result {
						re := regexp.MustCompile(pattern)
						if re.MatchString(value) {
							return cmp.ResultSuccess
						}
						return cmp.ResultFailure(
							fmt.Sprintf("%q did not match pattern %q", value, pattern))
					}
				}
				assert.Assert(t, regex_match(dnsResult.String(), expectedResults[i]))
			}
		}
	})
}

type TestResponseWriter struct {
}

// LocalAddr returns the net.Addr of the server
func (r TestResponseWriter) LocalAddr() net.Addr {
	return new(TestAddr)
}

// RemoteAddr returns the net.Addr of the client that sent the current request.
func (r TestResponseWriter) RemoteAddr() net.Addr {
	return new(TestAddr)
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
