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
