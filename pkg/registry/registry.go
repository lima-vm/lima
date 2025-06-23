// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/external/client"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
	"github.com/sirupsen/logrus"
)

type ExternalDriver struct {
	Name         string
	InstanceName string
	Command      *exec.Cmd
	SocketPath   string
	Client       *client.DriverClient // Client is the gRPC client for the external driver
	Path         string
	Ctx          context.Context
	Logger       *logrus.Logger
	CancelFunc   context.CancelFunc
}

var (
	internalDrivers = make(map[string]driver.Driver)
	ExternalDrivers = make(map[string]*ExternalDriver)
)

func List() map[string]string {
	if err := discoverDrivers(); err != nil {
		logrus.Warnf("Error discovering drivers: %v", err)
	}

	vmTypes := make(map[string]string)
	for name := range internalDrivers {
		vmTypes[name] = "internal"
	}
	for name, d := range ExternalDrivers {
		vmTypes[name] = d.Path
	}
	return vmTypes
}

func Get(name string) (*ExternalDriver, driver.Driver, bool) {
	if err := discoverDrivers(); err != nil {
		logrus.Warnf("Error discovering drivers: %v", err)
	}

	internalDriver, exists := internalDrivers[name]
	if !exists {
		externalDriver, exists := ExternalDrivers[name]
		if exists {
			return externalDriver, nil, exists
		}
	}
	return nil, internalDriver, exists
}

func registerExternalDriver(name, path string) {
	if _, exists := ExternalDrivers[name]; exists {
		return
	}

	if _, exists := internalDrivers[name]; exists {
		logrus.Warnf("Driver %q is already registered as an internal driver, skipping external registration", name)
		return
	}

	ExternalDrivers[name] = &ExternalDriver{
		Name:   name,
		Path:   path,
		Logger: logrus.New(),
	}
}

func discoverDrivers() error {
	prefix, err := usrlocalsharelima.Prefix()
	if err != nil {
		return err
	}
	stdDriverDir := filepath.Join(prefix, "libexec", "lima")

	logrus.Infof("Discovering drivers in %s", stdDriverDir)
	if _, err := os.Stat(stdDriverDir); err == nil {
		if err := discoverDriversInDir(stdDriverDir); err != nil {
			logrus.Warnf("Error discovering drivers in %q: %v", stdDriverDir, err)
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
				logrus.Warnf("Error accessing driver path %q: %v", path, err)
				continue
			}

			if info.IsDir() {
				if err := discoverDriversInDir(path); err != nil {
					logrus.Warnf("Error discovering drivers in %q: %v", path, err)
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
		return fmt.Errorf("failed to read driver directory %q: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			logrus.Warnf("Failed to get info for %q: %v", entry.Name(), err)
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
		return
	}

	name := strings.TrimPrefix(base, "lima-driver-")

	registerExternalDriver(name, path)
}

func isExecutable(mode os.FileMode) bool {
	return mode&0111 != 0
}

func Register(driver driver.Driver) {
	name := driver.Info().DriverName
	if _, exists := internalDrivers[name]; exists {
		return
	}
	internalDrivers[name] = driver
}
