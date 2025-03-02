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

package client

import (
	"context"
	"net"

	"google.golang.org/grpc/credentials"
)

// NewCredentials returns a lima credential implementing credentials.TransportCredentials.
func NewCredentials() credentials.TransportCredentials {
	return &secureTC{
		info: credentials.ProtocolInfo{
			SecurityProtocol: "local",
		},
	}
}

// secureTC is the credentials required to establish a lima guest connection.
type secureTC struct {
	info credentials.ProtocolInfo
}

func (c *secureTC) Info() credentials.ProtocolInfo {
	return c.info
}

func (*secureTC) ClientHandshake(_ context.Context, _ string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, info{credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity}}, nil
}

func (*secureTC) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, info{credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity}}, nil
}

func (c *secureTC) Clone() credentials.TransportCredentials {
	return &secureTC{info: c.info}
}

func (c *secureTC) OverrideServerName(serverNameOverride string) error {
	c.info.ServerName = serverNameOverride
	return nil
}

type info struct {
	credentials.CommonAuthInfo
}

func (info) AuthType() string {
	return "local"
}
