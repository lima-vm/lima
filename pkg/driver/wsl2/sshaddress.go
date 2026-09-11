// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"net"
	"strings"
)

// routableGuestIP returns the trimmed output when it is a routable IP address,
// or "" otherwise. The output comes from a command run inside the WSL2 guest
// and becomes inst.SSHAddress, which is passed to ssh/scp/rsync as the
// destination argument; a non-address value such as "-oProxyCommand=..." would
// be interpreted by ssh as an option, so it is validated here. Loopback is
// rejected because some distributions report 127.0.1.1, which is not routable.
func routableGuestIP(out []byte) string {
	s := strings.TrimSpace(string(out))
	if ip := net.ParseIP(s); ip == nil || ip.IsLoopback() {
		return ""
	}
	return s
}
