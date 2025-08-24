// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
	"github.com/lima-vm/lima/v2/pkg/driver/external/client"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/store/filenames"
)

type DriverServer struct {
	pb.UnimplementedDriverServer
	driver driver.Driver
	logger *logrus.Logger
}

type listenerTracker struct {
	net.Listener
	connected chan struct{}
	once      sync.Once
}

func (t *listenerTracker) Accept() (net.Conn, error) {
	c, err := t.Listener.Accept()
	if err == nil {
		t.once.Do(func() { close(t.connected) })
	}
	return c, err
}

func Serve(ctx context.Context, driver driver.Driver) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("lima-driver-%s-%d.sock", driver.Info(ctx).DriverName, os.Getpid()))

	defer func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("Failed to remove socket file: %v", err)
		}
	}()

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		logger.Fatalf("Failed to remove existing socket file: %v", err)
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "unix", socketPath)
	if err != nil {
		logger.Fatalf("Failed to listen on Unix socket: %v", err)
	}
	defer listener.Close()

	tListener := &listenerTracker{
		Listener:  listener,
		connected: make(chan struct{}),
	}

	output := map[string]string{"socketPath": socketPath}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		logger.Fatalf("Failed to encode socket path as JSON: %v", err)
	}

	kaProps := keepalive.ServerParameters{
		Time:    10 * time.Second,
		Timeout: 30 * time.Second,
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

	shutdownCh := make(chan struct{})
	var closeOnce sync.Once
	closeShutdown := func() { closeOnce.Do(func() { close(shutdownCh) }) }

	go func() {
		<-sigs
		logger.Info("Received shutdown signal, stopping server...")
		closeShutdown()
	}()

	go func() {
		timer := time.NewTimer(60 * time.Second)
		defer timer.Stop()

		select {
		case <-tListener.connected:
			logger.Debug("Client connected; disabling 60s startup shutdown")
			return
		case <-timer.C:
			logger.Info("No client connected within 60 seconds, shutting down server...")
			closeShutdown()
		case <-shutdownCh:
			return
		}
	}()

	go func(ctx context.Context) {
		logger.Infof("Starting external driver server for %s", driver.Info(ctx).DriverName)
		logger.Infof("Server starting on Unix socket: %s", socketPath)
		if err := server.Serve(tListener); err != nil {
			if errors.Is(err, grpc.ErrServerStopped) {
				logger.Errorf("Server stopped: %v", err)
			} else {
				logger.Errorf("Failed to serve: %v", err)
			}
		}
	}(ctx)

	<-shutdownCh
	server.GracefulStop()
}

func Start(extDriver *registry.ExternalDriver, instName string) error {
	extDriver.Logger.Debugf("Starting external driver at %s", extDriver.Path)
	if instName == "" {
		return errors.New("instance name cannot be empty")
	}
	extDriver.InstanceName = instName

	ctx, cancel := context.WithCancel(extDriver.Ctx)
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
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to open external driver log file: %w", err)
	}

	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start external driver: %w", err)
	}

	driverLogger := extDriver.Logger.WithField("driver", extDriver.Name)

	scanner := bufio.NewScanner(stdout)
	var socketPath string
	if scanner.Scan() {
		var output map[string]string
		if err := json.Unmarshal(scanner.Bytes(), &output); err != nil {
			cancel()
			if err := cmd.Process.Kill(); err != nil {
				driverLogger.Errorf("Failed to kill external driver process: %v", err)
			}
			return fmt.Errorf("failed to parse socket path JSON: %w", err)
		}
		socketPath = output["socketPath"]
	} else {
		cancel()
		if err := cmd.Process.Kill(); err != nil {
			driverLogger.Errorf("Failed to kill external driver process: %v", err)
		}
		return errors.New("failed to read socket path from driver")
	}
	extDriver.SocketPath = socketPath

	driverClient, err := client.NewDriverClient(extDriver.SocketPath, extDriver.Logger)
	if err != nil {
		cancel()
		if err := cmd.Process.Kill(); err != nil {
			driverLogger.Errorf("Failed to kill external driver process after client creation failure: %v", err)
		}
		return fmt.Errorf("failed to create driver client: %w", err)
	}

	extDriver.Command = cmd
	extDriver.Client = driverClient
	extDriver.Ctx = ctx
	extDriver.CancelFunc = cancel

	driverLogger.Debugf("External driver %s started successfully", extDriver.Name)
	return nil
}

func Stop(extDriver *registry.ExternalDriver) error {
	if extDriver.Command == nil {
		return fmt.Errorf("external driver %s is not running", extDriver.Name)
	}

	extDriver.Logger.Debugf("Stopping external driver %s", extDriver.Name)
	if extDriver.CancelFunc != nil {
		extDriver.CancelFunc()
	}
	if err := extDriver.Command.Process.Kill(); err != nil {
		extDriver.Logger.Errorf("Failed to kill external driver process: %v", err)
	}
	if err := os.Remove(extDriver.SocketPath); err != nil && !os.IsNotExist(err) {
		extDriver.Logger.Warnf("Failed to remove socket file: %v", err)
	}

	extDriver.Command = nil
	extDriver.Client = nil
	extDriver.Ctx = nil
	extDriver.CancelFunc = nil

	extDriver.Logger.Debugf("External driver %s stopped successfully", extDriver.Name)
	return nil
}

func StopAllExternalDrivers() {
	for name, driver := range registry.ExternalDrivers {
		if driver.Command != nil && driver.Command.Process != nil {
			if err := Stop(driver); err != nil {
				logrus.Errorf("Failed to stop external driver %s: %v", name, err)
			} else {
				logrus.Debugf("External driver %s stopped successfully", name)
			}
		}
		delete(registry.ExternalDrivers, name)
	}
}
