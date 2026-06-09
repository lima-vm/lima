// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
	"github.com/lima-vm/lima/v2/pkg/driver/external/client"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/registry"
)

type DriverServer struct {
	pb.UnimplementedDriverServer
	driver driver.Driver
	logger *logrus.Logger
}

func Serve(ctx context.Context, driver driver.Driver) {
	preConfiguredDriverAction := flag.Bool("pre-driver-action", false, "Run pre-driver action before starting the gRPC server")
	inspectStatus := flag.Bool("inspect-status", false, "Inspect status of the driver")
	instDir := flag.String("inst-dir", "", "Instance directory for the driver to store the gRPC server socket path")
	flag.Parse() //nolint:revive // Serve is intended to be called from external driver's main()
	if *preConfiguredDriverAction {
		handlePreConfiguredDriverAction(ctx, driver)
		return
	}
	if *inspectStatus {
		handleInspectStatus(ctx, driver)
		return
	}
	if *instDir == "" {
		logrus.Errorf("Instance directory is required")
		return
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	driverInfo := driver.Info(ctx)
	driverName := driverInfo.Name
	if base := filepath.Base(os.Args[0]); strings.HasPrefix(base, "lima-driver-") {
		driverName = strings.TrimSuffix(strings.TrimPrefix(base, "lima-driver-"), ".exe")
	}
	socketPath := driverSocketPath(*instDir, driverName)
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

	serveErr := make(chan error, 1)
	go func() {
		logger.Infof("Starting external driver server for %s", driverInfo.Name)
		logger.Infof("Server starting on Unix socket: %s", socketPath)
		serveErr <- server.Serve(listener)
	}()

	select {
	case <-sigs:
		logger.Info("Received shutdown signal, stopping server...")
		server.GracefulStop()
		<-serveErr
	case err := <-serveErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Errorf("Failed to serve: %v", err)
		} else {
			logger.Infof("Server stopped")
		}
	}
}

func handleInspectStatus(ctx context.Context, driver driver.Driver) {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	var payload []byte
	if err := decoder.Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode instance payload from stdin: %v", err)
	}

	var inst limatype.Instance
	if err := inst.UnmarshalJSON(payload); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmarshal instance: %v", err)
	}

	status := driver.InspectStatus(ctx, &inst)
	inst.Status = status

	resp, err := inst.MarshalJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal instance response: %v", err)
	}

	if err := encoder.Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode instance response: %v", err)
	}
}

func handlePreConfiguredDriverAction(ctx context.Context, driver driver.Driver) {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	var payload limatype.PreConfiguredDriverPayload
	if err := decoder.Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode pre-configured driver payload from stdin: %v", err)
	}

	config := &payload.Config
	if err := driver.FillConfig(ctx, config, payload.FilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fill config: %v", err)
	}

	if err := encoder.Encode(*config); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding response: %v", err)
	}
}

func driverPIDFilePath(instanceDir, driverName string) string {
	return filepath.Join(instanceDir, fmt.Sprintf("%s.drv.pid", driverName))
}

func driverSocketPath(instanceDir, driverName string) string {
	// The full path of a Unix domain socket is limited (see osutil.UnixPathMax),
	// so keep this file name as short as possible.
	return filepath.Join(instanceDir, fmt.Sprintf("%s.drv.sock", driverName))
}

// isServerRunning checks if an existing server is accessible on the socket.
func isServerRunning(socketPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Start connects to an existing external driver server if one is already running
// (socket exists and is connectable). Otherwise, it launches a new server process.
// Only one server process per instance should exist at any time.
func Start(ctx context.Context, extDriver *registry.ExternalDriver, instName string) error {
	extDriver.Logger.Debugf("Starting external driver at %s", extDriver.Path)
	if instName == "" {
		return errors.New("instance name cannot be empty")
	}
	extDriver.InstanceName = instName
	instanceDir, err := dirnames.InstanceDir(extDriver.InstanceName)
	if err != nil {
		return fmt.Errorf("failed to determine instance directory: %w", err)
	}
	socketPath := driverSocketPath(instanceDir, extDriver.Name)

	// If the socket already exists and is connectable, reuse the existing server.
	if isServerRunning(socketPath) {
		extDriver.Logger.Debugf("Reusing existing external driver server for %#q at %s", extDriver.Name, socketPath)
		extDriver.Client, err = client.NewDriverClient(socketPath, extDriver.Logger)
		if err != nil {
			return fmt.Errorf("failed to create driver client for existing server: %w", err)
		}
		return nil
	}

	// No running server found; start a new one.
	logPath := filepath.Join(instanceDir, filenames.ExternalDriverStderrLog)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open external driver log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	cmd := exec.CommandContext(ctx, extDriver.Path, "--inst-dir", instanceDir)
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start external driver: %w", err)
	}

	pid := cmd.Process.Pid
	pidFilePath := driverPIDFilePath(instanceDir, extDriver.Name)
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("failed to write driver PID file: %w", err)
	}

	driverLogger := extDriver.Logger.WithField("driver", extDriver.Name)

	procExit := make(chan error, 1)
	go func() {
		procExit <- cmd.Wait()
		close(procExit)
		_ = os.Remove(pidFilePath)
	}()

	// Wait for the socket file to be created by the external driver.
	socketWaitCtx, socketWaitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer socketWaitCancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(socketPath); err == nil && isServerRunning(socketPath) {
				driverLogger.Debugf("Detected socket file at %s", socketPath)
				extDriver.Client, err = client.NewDriverClient(socketPath, extDriver.Logger)
				if err != nil {
					if err := cmd.Process.Kill(); err != nil {
						driverLogger.Errorf("Failed to kill external driver process after client creation failure: %v", err)
					}
					<-procExit
					return fmt.Errorf("failed to create driver client: %w", err)
				}
				driverLogger.Debugf("External driver %s started successfully", extDriver.Name)
				return nil
			}
		case waitErr := <-procExit:
			if waitErr == nil {
				return errors.New("external driver process exited before creating socket file")
			}
			return fmt.Errorf("external driver process exited before creating socket file: %w", waitErr)
		case <-socketWaitCtx.Done():
			if err := cmd.Process.Kill(); err != nil {
				driverLogger.Errorf("Failed to kill external driver process after socket wait timeout: %v", err)
			}
			<-procExit
			return errors.New("timed out waiting for external driver to create socket file")
		}
	}
}

// Stop finds and stops any external driver server processes in the given
// instance directory using PID files. If force is true, SIGKILL is sent;
// otherwise SIGTERM is sent and we wait for the process to exit.
// Also cleans up PID files and socket files.
func Stop(instDir string, force bool) {
	logrus.Infof("Stopping external driver server in instance directory %s", instDir)
	pidPattern := filepath.Join(instDir, "*.drv.pid")
	files, _ := filepath.Glob(pidPattern)
	for _, pidFile := range files {
		pidData, err := os.ReadFile(pidFile)
		if err != nil {
			_ = os.Remove(pidFile)
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
		if err != nil || pid <= 0 {
			_ = os.Remove(pidFile)
			continue
		}
		// Check if process is still alive.
		if !osutil.ProcessAlive(pid) {
			_ = os.Remove(pidFile)
			continue
		}
		if force {
			logrus.Infof("Sending SIGKILL to external driver process %d", pid)
			_ = osutil.SysKill(pid, osutil.SigKill)
		} else {
			logrus.Infof("Sending SIGTERM to external driver process %d", pid)
			_ = osutil.SysKill(pid, osutil.SigTerm)
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				if !osutil.ProcessAlive(pid) {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if osutil.ProcessAlive(pid) {
				logrus.Warnf("External driver process %d did not exit after SIGTERM; sending SIGKILL", pid)
				_ = osutil.SysKill(pid, osutil.SigKill)
			}
		}
		_ = os.Remove(pidFile)
	}
	// Clean up leftover sockets.
	sockPattern := filepath.Join(instDir, "*.drv.sock")
	sockFiles, _ := filepath.Glob(sockPattern)
	for _, sockFile := range sockFiles {
		_ = os.Remove(sockFile)
	}
}

// Disconnect closes gRPC client connections for all external drivers in the
// registry without stopping the server processes.
func Disconnect() {
	for _, d := range registry.ExternalDrivers {
		if d.Client != nil && d.Client.Conn != nil {
			d.Logger.Debugf("Disconnecting gRPC client for external driver %#q", d.Name)
			_ = d.Client.Conn.Close()
			d.Client = nil
		}
	}
}
