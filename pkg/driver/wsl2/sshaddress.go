// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"net"
	"strings"
)

// routableGuestIP returns the trimmed output when it is a global unicast IP
// address (private and ULA ranges included), or "" otherwise. The output comes
// from a command run inside the WSL2 guest and becomes inst.SSHAddress, which
// is passed to ssh/scp/rsync as the destination argument, so anything that is
// not an address we can connect to is rejected here and the caller falls
// through to the next probe. Besides non-address output this rejects the
// unspecified, loopback (some distributions report 127.0.1.1), link-local and
// multicast addresses.
func routableGuestIP(out []byte) string {
	s := strings.TrimSpace(string(out))
	if ip := net.ParseIP(s); ip == nil || !ip.IsGlobalUnicast() {
		return ""
	}
	return s
}
