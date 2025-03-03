// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net"
	"strconv"
)

func (x *IPPort) HostString() string {
	return net.JoinHostPort(x.Ip, strconv.Itoa(int(x.Port)))
}
