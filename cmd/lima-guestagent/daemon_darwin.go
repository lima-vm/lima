// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdlayher/vsock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/guestagent"
	"github.com/lima-vm/lima/v2/pkg/guestagent/api/server"
	"github.com/lima-vm/lima/v2/pkg/guestagent/ticker"
	"github.com/lima-vm/lima/v2/pkg/portfwdserver"
)

const hostCID = 2

type cidFilteredListener struct {
	*vsock.Listener
}

func (l *cidFilteredListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if vsockConn, ok := conn.(*vsock.Conn); ok {
			if addr, ok := vsockConn.RemoteAddr().(*vsock.Addr); ok {
				if addr.ContextID != hostCID {
					logrus.Warnf("rejected vsock connection from unauthorized CID %d", addr.ContextID)
					conn.Close()
					continue
				}
			}
		}
		return conn, nil
	}
}

func newDaemonCommand() *cobra.Command {
	daemonCommand := &cobra.Command{
		Use:   "daemon",
		Short: "Run the daemon",
		RunE:  daemonAction,
	}
	daemonCommand.Flags().String("runtime-dir", "/var/run/lima-guestagent", "Directory to store runtime state")
	daemonCommand.Flags().Duration("tick", 3*time.Second, "Tick for polling events")
	daemonCommand.Flags().Int("vsock-port", 0, "vsock port to listen on")
	return daemonCommand
}

func daemonAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	runtimeDir, err := cmd.Flags().GetString("runtime-dir")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return err
	}
	tick, err := cmd.Flags().GetDuration("tick")
	if err != nil {
		return err
	}
	vSockPort, err := cmd.Flags().GetInt("vsock-port")
	if err != nil {
		return err
	}
	if tick == 0 {
		return errors.New("tick must be specified")
	}
	if vSockPort == 0 {
		return errors.New("vsock-port must be specified for macOS guests")
	}
	if os.Geteuid() != 0 {
		return errors.New("must run as the root user")
	}

	logrus.Infof("event tick: %v", tick)
	tickerInst := ticker.NewSimpleTicker(time.NewTicker(tick))

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		logrus.Debug("Received SIGTERM, shutting down the guest agent")
	}()

	agent, err := guestagent.New(ctx, tickerInst, runtimeDir)
	if err != nil {
		return err
	}

	vsockL, err := vsock.Listen(uint32(vSockPort), nil)
	if err != nil {
		return err
	}
	l := &cidFilteredListener{Listener: vsockL}
	logrus.Infof("serving the guest agent on vsock port: %d (host CID only)", vSockPort)

	defer logrus.Debug("exiting lima-guestagent daemon")
	return server.StartServer(ctx, l, &server.GuestServer{Agent: agent, TunnelS: portfwdserver.NewTunnelServer()})
}
