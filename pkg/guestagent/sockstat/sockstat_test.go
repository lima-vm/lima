package sockstat

import (
	"net"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseFreeBSD(t *testing.T) {
	sockstat := `USER     COMMAND    PID   FD  PROTO  LOCAL ADDRESS         FOREIGN ADDRESS
root     sshd         831 3   tcp6   *:22                  *:*
root     sshd         831 4   tcp4   *:22                  *:*
`
	entries, err := Parse(strings.NewReader(sockstat), Listen)
	assert.NilError(t, err)
	t.Log(entries)

	assert.Check(t, net.ParseIP("::").Equal(entries[0].IP))
	assert.Equal(t, uint16(22), entries[0].Port)
	assert.Equal(t, TCP6, entries[0].Kind)

	assert.Check(t, net.ParseIP("0.0.0.0").Equal(entries[1].IP))
	assert.Equal(t, uint16(22), entries[1].Port)
	assert.Equal(t, TCP4, entries[1].Kind)
}
