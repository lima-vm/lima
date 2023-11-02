package usernet

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
)

func TestIncIP(t *testing.T) {
	cases := []struct {
		ip   string
		want string
	}{
		{ip: "0.0.0.0", want: "0.0.0.1"},
		{ip: "10.0.0.0", want: "10.0.0.1"},
		{ip: "192.168.1.1", want: "192.168.1.2"},
		{ip: "9.255.255.255", want: "10.0.0.0"},
		{ip: "255.255.255.255", want: "0.0.0.0"},
		{ip: "::", want: "::1"},
		{ip: "::1", want: "::2"},
		{ip: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", want: "::"},
		{ip: "2001:0db8::1", want: "2001:0db8::2"},
		{ip: "2001:db8:c001:ba00::", want: "2001:db8:c001:ba00::1"},
		{ip: "2001:0db8:85a3:0000:0000:8a2e:0370:7334", want: "2001:0db8:85a3:0000:0000:8a2e:0370:7335"},
	}
	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			input := net.ParseIP(c.ip)
			want := net.ParseIP(c.want)
			got := incIP(input)
			assert.Assert(t, got.Equal(want))
		})
	}
}
