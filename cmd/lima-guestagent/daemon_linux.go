// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/mdlayher/vsock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/guestagent"
	"github.com/lima-vm/lima/v2/pkg/guestagent/api/server"
	"github.com/lima-vm/lima/v2/pkg/guestagent/serialport"
	"github.com/lima-vm/lima/v2/pkg/guestagent/ticker"
	"github.com/lima-vm/lima/v2/pkg/portfwdserver"
)

func newDaemonCommand() *cobra.Command {
	daemonCommand := &cobra.Command{
		Use:   "daemon",
		Short: "Run the daemon",
		RunE:  daemonAction,
	}
	daemonCommand.Flags().Duration("tick", 3*time.Second, "Tick for polling events")
	daemonCommand.Flags().Int("vsock-port", 0, "Use vsock server instead a UNIX socket")
	daemonCommand.Flags().String("virtio-port", "", "Use virtio server instead a UNIX socket")
	return daemonCommand
}

func daemonAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	socket := "/run/lima-guestagent.sock"
	tick, err := cmd.Flags().GetDuration("tick")
	if err != nil {
		return err
	}
	vSockPort, err := cmd.Flags().GetInt("vsock-port")
	if err != nil {
		return err
	}
	virtioPort, err := cmd.Flags().GetString("virtio-port")
	if err != nil {
		return err
	}
	if tick == 0 {
		return errors.New("tick must be specified")
	}
	if os.Geteuid() != 0 {
		return errors.New("must run as the root user")
	}

	logrus.Infof("event tick: %v", tick)
	simpleTicker := ticker.NewSimpleTicker(time.NewTicker(tick))
	tickerInst := simpleTicker
	// See /sys/kernel/debug/tracing/available_events for the list of available tracepoints
	tracepoints := []string{"syscalls:sys_exit_bind"}
	if ebpfTicker, err := ticker.NewEbpfTicker(tracepoints); err != nil {
		logrus.WithError(err).Warn("failed to create eBPF ticker, falling back to simple ticker")
	} else {
		logrus.Infof("using eBPF ticker with tracepoints: %v", tracepoints)
		tickerInst = ticker.NewCompoundTicker(simpleTicker, ebpfTicker)
	}

	agent, err := guestagent.New(ctx, tickerInst, tick*20)
	if err != nil {
		return err
	}
	err = os.RemoveAll(socket)
	if err != nil {
		return err
	}

	var l net.Listener
	if virtioPort != "" {
		qemuL, err := serialport.Listen("/dev/virtio-ports/" + virtioPort)
		if err != nil {
			return err
		}
		l = qemuL
		logrus.Infof("serving the guest agent on qemu serial file: %s", virtioPort)
	} else if vSockPort != 0 {
		vsockL, err := vsock.Listen(uint32(vSockPort), nil)
		if err != nil {
			return err
		}
		l = vsockL
		logrus.Infof("serving the guest agent on vsock port: %d", vSockPort)
	} else {
		var lc net.ListenConfig
		socketL, err := lc.Listen(ctx, "unix", socket)
		if err != nil {
			return err
		}
		if err := os.Chmod(socket, 0o777); err != nil {
			return err
		}
		l = socketL
		logrus.Infof("serving the guest agent on %q", socket)
	}
	return server.StartServer(l, &server.GuestServer{Agent: agent, TunnelS: portfwdserver.NewTunnelServer()})
}
