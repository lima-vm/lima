package procnettcp

import (
	"net"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseTCP(t *testing.T) {
	procNetTCP := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode                                                     
   0: 0100007F:8AEF 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 28152 1 0000000000000000 100 0 0 10 0                     
   1: 0103000A:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 31474 1 0000000000000000 100 0 0 10 5                     
   2: 3500007F:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000   102        0 30955 1 0000000000000000 100 0 0 10 0                     
   3: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 32910 1 0000000000000000 100 0 0 10 0                     
   4: 0100007F:053A 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 31430 1 0000000000000000 100 0 0 10 0                     
   5: 0B3CA8C0:0016 690AA8C0:F705 01 00000000:00000000 02:00028D8B 00000000     0        0 32989 4 0000000000000000 20 4 31 10 19
`
	entries, err := Parse(strings.NewReader(procNetTCP), TCP)
	assert.NilError(t, err)
	t.Log(entries)

	assert.Check(t, net.ParseIP("127.0.0.1").Equal(entries[0].IP))
	assert.Equal(t, uint16(35567), entries[0].Port)
	assert.Equal(t, TCPListen, entries[0].State)

	assert.Check(t, net.ParseIP("192.168.60.11").Equal(entries[5].IP))
	assert.Equal(t, uint16(22), entries[5].Port)
	assert.Equal(t, TCPEstablished, entries[5].State)
}

func TestParseTCP6(t *testing.T) {
	procNetTCP := `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
	   0: 000080FE00000000FF57A6705DC771FE:0050 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 850222 1 0000000000000000 100 0 0 10 0`
	entries, err := Parse(strings.NewReader(procNetTCP), TCP6)
	assert.NilError(t, err)
	t.Log(entries)

	assert.Check(t, net.ParseIP("fe80::70a6:57ff:fe71:c75d").Equal(entries[0].IP))
	assert.Equal(t, uint16(80), entries[0].Port)
	assert.Equal(t, TCPListen, entries[0].State)
}

func TestParseTCP6Zero(t *testing.T) {
	procNetTCP := `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:0016 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 33825 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:006F 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 26772 1 0000000000000000 100 0 0 10 0
   2: 00000000000000000000000000000000:0050 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1210901 1 0000000000000000 100 0 0 10 0
`
	entries, err := Parse(strings.NewReader(procNetTCP), TCP6)
	assert.NilError(t, err)
	t.Log(entries)

	assert.Check(t, net.IPv6zero.Equal(entries[0].IP))
	assert.Equal(t, uint16(22), entries[0].Port)
	assert.Equal(t, TCPListen, entries[0].State)
}

func TestParseUDP(t *testing.T) {
	procNetTCP := `   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops            
  716: 3600007F:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000   991        0 2964 2 0000000000000000 0          
  716: 3500007F:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000   991        0 2962 2 0000000000000000 0          
  731: 0369A8C0:0044 00000000:0000 07 00000000:00000000 00:00000000 00000000   998        0 29132 2 0000000000000000 0         
  731: 0F05A8C0:0044 00000000:0000 07 00000000:00000000 00:00000000 00000000   998        0 4049 2 0000000000000000 0          
 1768: 00000000:1451 00000000:0000 07 00000000:00000000 00:00000000 00000000   502        0 28364 2 0000000000000000 0  `
	entries, err := Parse(strings.NewReader(procNetTCP), UDP)
	assert.NilError(t, err)
	t.Log(entries)

	assert.Check(t, net.ParseIP("127.0.0.54").Equal(entries[0].IP))
	assert.Equal(t, uint16(53), entries[0].Port)
	assert.Equal(t, UDPEstablished, entries[0].State)
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		input             string
		expectedIP        net.IP
		expectedPort      uint16
		expectedErrSubstr string
	}{
		{
			input:        "0100007F:0050",
			expectedIP:   net.IPv4(127, 0, 0, 1),
			expectedPort: 80,
		},
		{
			input:        "000080FE00000000FF57A6705DC771FE:0050",
			expectedIP:   net.ParseIP("fe80::70a6:57ff:fe71:c75d"),
			expectedPort: 80,
		},
		{
			input:        "00000000000000000000000000000000:0050",
			expectedIP:   net.IPv6zero,
			expectedPort: 80,
		},
		{
			input:             "0100007F",
			expectedErrSubstr: `unparsable address "0100007F"`,
		},
		{
			input:             "invalid:address",
			expectedErrSubstr: `unparsable address "invalid:address", expected length of`,
		},
		{
			input:             "0100007F:0050:00",
			expectedErrSubstr: `unparsable address "0100007F:0050:00"`,
		},
		{
			input:             "0100007G:0050", // Invalid hex character 'G'
			expectedErrSubstr: `unparsable address "0100007G:0050": unparsable quartet "0100007G"`,
		},
		{
			input:             "0100007F:",
			expectedErrSubstr: `unparsable address "0100007F:": unparsable port ""`,
		},
		{
			input:             "0100007F:invalid",
			expectedErrSubstr: `unparsable address "0100007F:invalid": unparsable port "invalid"`,
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			ip, port, err := ParseAddress(test.input)
			if test.expectedErrSubstr != "" {
				assert.ErrorContains(t, err, test.expectedErrSubstr)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, test.expectedIP, ip)
				assert.Equal(t, test.expectedPort, port)
			}
		})
	}
}
