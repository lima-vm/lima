// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/external/client"
	"github.com/sirupsen/logrus"
)

type ExternalDriver struct {
	Name       string
	Command    *exec.Cmd
	Stdin      io.WriteCloser
	Stdout     io.ReadCloser
	Client     *client.DriverClient // Client is the gRPC client for the external driver
	Path       string
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type Registry struct {
	internalDrivers map[string]driver.Driver
	externalDrivers map[string]*ExternalDriver
	mu              sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		internalDrivers: make(map[string]driver.Driver),
		externalDrivers: make(map[string]*ExternalDriver),
	}
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.internalDrivers {
		names = append(names, name)
	}

	r.DiscoverDrivers()
	for name := range r.externalDrivers {
		names = append(names, name+" (external)")
	}
	return names
}

func (r *Registry) Get(name string) (driver.Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.internalDrivers[name]
	if !exists {
		externalDriver, exists := r.externalDrivers[name]
		if exists {
			return externalDriver.Client, true
		}

	}
	return driver, exists
}

func (r *Registry) RegisterDriver(name, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.externalDrivers[name]; exists {
		logrus.Debugf("Driver %q is already registered, skipping", name)
		return
	}

	r.externalDrivers[name].Path = path
	logrus.Debugf("Registered driver %q at %s", name, path)
}

func (r *Registry) DiscoverDrivers() error {
	// limaShareDir, err := usrlocalsharelima.Dir()
	// if err != nil {
	// 	return fmt.Errorf("failed to determine Lima share directory: %w", err)
	// }
	// fmt.Printf("Discovering drivers in %s\n", limaShareDir)
	// stdDriverDir := filepath.Join(filepath.Dir(limaShareDir), "libexec", "lima", "drivers")

	// if _, err := os.Stat(stdDriverDir); err == nil {
	// 	if err := r.discoverDriversInDir(stdDriverDir); err != nil {
	// 		logrus.Warnf("Error discovering drivers in %s: %v", stdDriverDir, err)
	// 	}
	// }

	if driverPaths := os.Getenv("LIMA_DRIVERS_PATH"); driverPaths != "" {
		fmt.Printf("Discovering drivers in LIMA_DRIVERS_PATH: %s\n", driverPaths)
		paths := filepath.SplitList(driverPaths)
		fmt.Println("Driver paths:", paths)
		for _, path := range paths {
			if path == "" {
				continue
			}

			info, err := os.Stat(path)
			if err != nil {
				logrus.Warnf("Error accessing driver path %s: %v", path, err)
				continue
			}
			fmt.Printf("Info for %s: %+v\n", path, info)
			fmt.Printf("IsExecutable: %v\n", isExecutable(info.Mode()))
			fmt.Printf("IsDir: %v\n", info.IsDir())

			if info.IsDir() {
				if err := r.discoverDriversInDir(path); err != nil {
					logrus.Warnf("Error discovering drivers in %s: %v", path, err)
				}
			} else if isExecutable(info.Mode()) {
				r.registerDriverFile(path)
			}
		}
	}

	return nil
}

func (r *Registry) discoverDriversInDir(dir string) error {
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
		r.registerDriverFile(driverPath)
	}

	return nil
}

func (r *Registry) registerDriverFile(path string) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "lima-driver-") {
		return
	}

	name := strings.TrimPrefix(base, "lima-driver-")
	name = strings.TrimSuffix(name, filepath.Ext(name))

	cmd := exec.Command(path, "--version")
	if err := cmd.Run(); err != nil {
		logrus.Warnf("driver %s failed version check: %v", path, err)
		return
	}

	r.RegisterDriver(name, path)
}

func isExecutable(mode os.FileMode) bool {
	return mode&0111 != 0
}

var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry()
	DefaultRegistry.DiscoverDrivers()
}

func Register(driver driver.Driver) {
	if DefaultRegistry != nil {
		name := driver.GetInfo().DriverName
		if _, exists := DefaultRegistry.internalDrivers[name]; exists {
			return
		}

		DefaultRegistry.internalDrivers[name] = driver
	}
}
