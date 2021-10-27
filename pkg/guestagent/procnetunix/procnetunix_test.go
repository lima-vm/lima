package procnetunix

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestParse(t *testing.T) {
	procNetUnix := `Num       RefCount Protocol Flags    Type St Inode Path
0000000000000000: 00000002 00000000 00000000 0002 01 21323 /run/user/501/systemd/notify
0000000000000000: 00000002 00000000 00010000 0001 01 21326 /run/user/501/systemd/private
0000000000000000: 00000002 00000000 00000000 0002 01 20264
0000000000000000: 00000003 00000000 00000000 0001 03 19605 /run/dbus/system_bus_socket
`
	entries, err := Parse(strings.NewReader(procNetUnix))
	assert.NilError(t, err)
	t.Log(entries)

	assert.Equal(t, "/run/user/501/systemd/notify", entries[0].Path)
	assert.Equal(t, StateUnconnected, entries[0].State)

	assert.Equal(t, "/run/dbus/system_bus_socket", entries[2].Path)
	assert.Equal(t, StateConnected, entries[2].State)
}
