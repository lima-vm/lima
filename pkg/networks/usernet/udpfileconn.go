/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package usernet

import (
	"errors"
	"net"
	"time"
)

type UDPFileConn struct {
	net.Conn
}

func (conn *UDPFileConn) Read(b []byte) (n int, err error) {
	// Check if the connection has been closed
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
			return 0, errors.New("UDPFileConn connection closed")
		}
	}
	return conn.Conn.Read(b)
}
