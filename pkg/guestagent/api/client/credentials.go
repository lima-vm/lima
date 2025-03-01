// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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
