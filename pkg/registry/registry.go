// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/external/client"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

type ExternalDriver struct {
	Name         string
	InstanceName string
	Command      *exec.Cmd
	SocketPath   string
	Client       *client.DriverClient // Client is the gRPC client for the external driver
	Path         string
	ctx          context.Context
	logger       *logrus.Logger
	cancelFunc   context.CancelFunc
}

var (
	internalDrivers = make(map[string]driver.Driver)
	externalDrivers = make(map[string]*ExternalDriver)
)

func (e *ExternalDriver) Start(instName string) error {
	e.logger.Infof("Starting external driver at %s", e.Path)
	if instName == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	e.InstanceName = instName

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, e.Path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe for external driver: %w", err)
	}

	instanceDir, err := store.InstanceDir(e.InstanceName)
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

	driverLogger := e.logger.WithField("driver", e.Name)

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
	e.SocketPath = socketPath

	driverClient, err := client.NewDriverClient(e.SocketPath, e.logger)
	if err != nil {
		cancel()
		cmd.Process.Kill()
		return fmt.Errorf("failed to create driver client: %w", err)
	}

	e.Command = cmd
	e.Client = driverClient
	e.ctx = ctx
	e.cancelFunc = cancel

	driverLogger.Infof("External driver %s started successfully", e.Name)
	return nil
}

func (e *ExternalDriver) cleanup() {
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	if err := os.Remove(e.SocketPath); err != nil && !os.IsNotExist(err) {
		e.logger.Warnf("Failed to remove socket file: %v", err)
	}

	e.Command = nil
	e.Client = nil
	e.ctx = nil
	e.cancelFunc = nil
}

func (e *ExternalDriver) Stop() error {
	if e.Command == nil || e.Command.Process == nil {
		return fmt.Errorf("external driver %s is not running", e.Name)
	}

	e.logger.Infof("Stopping external driver %s", e.Name)
	e.cleanup()

	e.logger.Infof("External driver %s stopped successfully", e.Name)
	return nil
}

func StopAllExternalDrivers() {
	for name, driver := range externalDrivers {
		if driver.Command != nil && driver.Command.Process != nil {
			if err := driver.Stop(); err != nil {
				logrus.Errorf("Failed to stop external driver %s: %v", name, err)
			} else {
				logrus.Infof("External driver %s stopped successfully", name)
			}
		}
		delete(externalDrivers, name)
	}
}

func List() map[string]string {
	vmTypes := make(map[string]string)
	for name := range internalDrivers {
		vmTypes[name] = "internal"
	}
	for name, d := range externalDrivers {
		vmTypes[name] = d.Path
	}
	return vmTypes
}

func Get(name, instName string) (driver.Driver, bool) {
	driver, exists := internalDrivers[name]
	if !exists {
		externalDriver, exists := externalDrivers[name]
		if exists {
			externalDriver.logger.Debugf("Using external driver %q", name)
			if externalDriver.Client == nil || externalDriver.Command == nil || externalDriver.Command.Process == nil {
				logrus.Infof("Starting new instance of external driver %q", name)
				if err := externalDriver.Start(instName); err != nil {
					externalDriver.logger.Errorf("Failed to start external driver %q: %v", name, err)
					return nil, false
				}
			} else {
				logrus.Infof("Reusing existing external driver %q instance", name)
				externalDriver.InstanceName = instName
			}
			return externalDriver.Client, true
		}
	}
	return driver, exists
}

func RegisterDriver(name, path string) {
	if _, exists := externalDrivers[name]; exists {
		logrus.Debugf("Driver %q is already registered, skipping", name)
		return
	}
	externalDrivers[name] = &ExternalDriver{
		Name:   name,
		Path:   path,
		logger: logrus.New(),
	}
}

func DiscoverDrivers() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stdDriverDir := filepath.Join(homeDir, ".local", "libexec", "lima", "drivers")

	logrus.Infof("Discovering drivers in %s", stdDriverDir)
	if _, err := os.Stat(stdDriverDir); err == nil {
		if err := discoverDriversInDir(stdDriverDir); err != nil {
			logrus.Warnf("Error discovering drivers in %s: %v", stdDriverDir, err)
		}
	}

	if driverPaths := os.Getenv("LIMA_DRIVERS_PATH"); driverPaths != "" {
		paths := filepath.SplitList(driverPaths)
		for _, path := range paths {
			if path == "" {
				continue
			}

			info, err := os.Stat(path)
			if err != nil {
				logrus.Warnf("Error accessing driver path %s: %v", path, err)
				continue
			}

			if info.IsDir() {
				if err := discoverDriversInDir(path); err != nil {
					logrus.Warnf("Error discovering drivers in %s: %v", path, err)
				}
			} else if isExecutable(info.Mode()) {
				registerDriverFile(path)
			}
		}
	}

	return nil
}

func discoverDriversInDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read driver directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			logrus.Warnf("Failed to get info for %s: %v", entry.Name(), err)
			continue
		}

		if !isExecutable(info.Mode()) {
			continue
		}

		driverPath := filepath.Join(dir, entry.Name())
		registerDriverFile(driverPath)
	}

	return nil
}

func registerDriverFile(path string) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "lima-driver-") {
		fmt.Printf("Skipping %s: does not start with 'lima-driver-'\n", base)
		return
	}

	name := strings.TrimPrefix(base, "lima-driver-")
	name = strings.TrimSuffix(name, filepath.Ext(name))

	RegisterDriver(name, path)
}

func isExecutable(mode os.FileMode) bool {
	return mode&0111 != 0
}

func init() {
	if err := DiscoverDrivers(); err != nil {
		logrus.Warnf("Error discovering drivers: %v", err)
	}
}

func Register(driver driver.Driver) {
	name := driver.Info().DriverName
	if _, exists := internalDrivers[name]; exists {
		return
	}
	internalDrivers[name] = driver
}
