// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/lima-vm/lima/pkg/driver"
	pb "github.com/lima-vm/lima/pkg/driver/external"
)

type DriverServer struct {
	pb.UnimplementedDriverServer
	driver driver.Driver
	logger *logrus.Logger
}

func Serve(driver driver.Driver) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Infof("Starting external driver server for %s", driver.GetInfo().DriverName)

	pipeConn := &PipeConn{
		Reader: os.Stdin,
		Writer: os.Stdout,
		Closer: os.Stdout,
	}

	listener := NewPipeListener(pipeConn)

	kaProps := keepalive.ServerParameters{
		Time:    10 * time.Second,
		Timeout: 20 * time.Second,
	}

	kaPolicy := keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}

	server := grpc.NewServer(
		grpc.KeepaliveParams(kaProps),
		grpc.KeepaliveEnforcementPolicy(kaPolicy),
	)

	pb.RegisterDriverServer(server, &DriverServer{
		driver: driver,
		logger: logger,
	})

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		logger.Info("Received shutdown signal, stopping server...")
		server.Stop()
		os.Exit(0)
	}()

	logger.Info("Server starting...")
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("Failed to serve: %v", err)
	}
}

func HandleProxyConnection(conn net.Conn, unixSocketPath string) {
	defer conn.Close()

	logrus.Infof("Handling proxy connection from %s", conn.LocalAddr())

	unixConn, err := net.Dial("unix", unixSocketPath)
	if err != nil {
		logrus.Errorf("Failed to connect to unix socket %s: %v", unixSocketPath, err)
		return
	}
	defer unixConn.Close()

	logrus.Infof("Successfully established proxy tunnel: %s <--> %s", conn.LocalAddr(), unixSocketPath)

	go func() {
		_, err := io.Copy(conn, unixConn)
		if err != nil {
			logrus.Errorf("Error copying from unix to vsock: %v", err)
		}
	}()

	_, err = io.Copy(unixConn, conn)
	if err != nil {
		logrus.Errorf("Error copying from vsock to unix: %v", err)
	}

	logrus.Infof("Proxy session ended for %s", conn.LocalAddr())

}
