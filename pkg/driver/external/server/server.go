// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/lima-vm/lima/pkg/driver"
	pb "github.com/lima-vm/lima/pkg/driver/external"
	"github.com/lima-vm/lima/pkg/driver/external/client"
	"github.com/lima-vm/lima/pkg/registry"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type DriverServer struct {
	pb.UnimplementedDriverServer
	driver driver.Driver
	logger *logrus.Logger
}

func Serve(driver driver.Driver) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("lima-driver-%s-%d.sock", driver.Info().DriverName, os.Getpid()))

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

	logger.Infof("Starting external driver server for %s", driver.Info().DriverName)
	logger.Infof("Server starting on Unix socket: %s", socketPath)
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("Failed to serve: %v", err)
	}
}

func Start(extDriver *registry.ExternalDriver, instName string) error {
	extDriver.Logger.Infof("Starting external driver at %s", extDriver.Path)
	if instName == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	extDriver.InstanceName = instName

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, extDriver.Path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe for external driver: %w", err)
	}

	instanceDir, err := store.InstanceDir(extDriver.InstanceName)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to determine instance directory: %w", err)
	}
	logPath := filepath.Join(instanceDir, filenames.ExternalDriverStderrLog)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to open external driver log file: %w", err)
	}

	// Redirect stderr to the log file
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start external driver: %w", err)
	}

	driverLogger := extDriver.Logger.WithField("driver", extDriver.Name)

	time.Sleep(time.Millisecond * 100)

	scanner := bufio.NewScanner(stdout)
	var socketPath string
	if scanner.Scan() {
		socketPath = strings.TrimSpace(scanner.Text())
	} else {
		cancel()
		cmd.Process.Kill()
		return fmt.Errorf("failed to read socket path from driver")
	}
	extDriver.SocketPath = socketPath

	driverClient, err := client.NewDriverClient(extDriver.SocketPath, extDriver.Logger)
	if err != nil {
		cancel()
		cmd.Process.Kill()
		return fmt.Errorf("failed to create driver client: %w", err)
	}

	extDriver.Command = cmd
	extDriver.Client = driverClient
	extDriver.Ctx = ctx
	extDriver.CancelFunc = cancel

	driverLogger.Infof("External driver %s started successfully", extDriver.Name)
	return nil
}

func Stop(extDriver *registry.ExternalDriver) error {
	if extDriver.Command == nil || extDriver.Command.Process == nil {
		return fmt.Errorf("external driver %s is not running", extDriver.Name)
	}

	extDriver.Logger.Infof("Stopping external driver %s", extDriver.Name)
	if extDriver.CancelFunc != nil {
		extDriver.CancelFunc()
	}
	if err := os.Remove(extDriver.SocketPath); err != nil && !os.IsNotExist(err) {
		extDriver.Logger.Warnf("Failed to remove socket file: %v", err)
	}

	extDriver.Command = nil
	extDriver.Client = nil
	extDriver.Ctx = nil
	extDriver.CancelFunc = nil

	extDriver.Logger.Infof("External driver %s stopped successfully", extDriver.Name)
	return nil
}

func StopAllExternalDrivers() {
	for name, driver := range registry.ExternalDrivers {
		if driver.Command != nil && driver.Command.Process != nil {
			if err := Stop(driver); err != nil {
				logrus.Errorf("Failed to stop external driver %s: %v", name, err)
			} else {
				logrus.Infof("External driver %s stopped successfully", name)
			}
		}
		delete(registry.ExternalDrivers, name)
	}
}
