// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
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

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("lima-driver-%s-%d.sock", driver.GetInfo().DriverName, os.Getpid()))

	defer func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("Failed to remove socket file: %v", err)
		}
	}()

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		logger.Fatalf("Failed to remove existing socket file: %v", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		logger.Fatalf("Failed to listen on Unix socket: %v", err)
	}
	defer listener.Close()

	fmt.Println(socketPath)

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
		server.GracefulStop()
		os.Exit(0)
	}()

	logger.Infof("Starting external driver server for %s", driver.GetInfo().DriverName)
	logger.Infof("Server starting on Unix socket: %s", socketPath)
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("Failed to serve: %v", err)
	}
}
