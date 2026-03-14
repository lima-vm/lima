//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"context"
	"net"
	"path/filepath"
)

func Listen(ctx context.Context, listenConfig net.ListenConfig, hostAddress string) (net.Listener, error) {
	if filepath.IsAbs(hostAddress) {
		// Handle Unix domain sockets
		if err := prepareUnixSocket(hostAddress); err != nil {
			return nil, err
		}
		var lc net.ListenConfig
		unixLis, err := lc.Listen(ctx, "unix", hostAddress)
		if err != nil {
			logListenError(err, "unix", hostAddress)
			return nil, err
		}
		return unixLis, nil
	}
	return listenConfig.Listen(ctx, "tcp", hostAddress)
}

func ListenPacket(ctx context.Context, listenConfig net.ListenConfig, hostAddress string) (net.PacketConn, error) {
	return listenConfig.ListenPacket(ctx, "udp", hostAddress)
}
