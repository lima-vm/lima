// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driver/external/client"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
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
	logger     *logrus.Logger
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

func (e *ExternalDriver) Start() error {
	e.logger.Infof("Starting external driver at %s", e.Path)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, e.Path)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start external driver: %w", err)
	}

	driverLogger := e.logger.WithField("driver", e.Name)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			driverLogger.Info(scanner.Text())
		}
	}()

	time.Sleep(4 * time.Second)

	driverClient, err := client.NewDriverClient(stdin, stdout, e.logger)
	if err != nil {
		cancel()
		cmd.Process.Kill()
		return fmt.Errorf("failed to create driver client: %w", err)
	}

	e.Command = cmd
	e.Stdin = stdin
	e.Stdout = stdout
	e.Client = driverClient
	e.ctx = ctx
	e.cancelFunc = cancel

	driverLogger.Infof("External driver %s started successfully", e.Name)
	return nil
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.internalDrivers {
		names = append(names, name)
	}

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
			externalDriver.logger.Debugf("Using external driver %q", name)
			if err := externalDriver.Start(); err != nil {
				externalDriver.logger.Errorf("Failed to start external driver %q: %v", name, err)
				return nil, false
			}
			return externalDriver.Client, true
		}

	}
	return driver, exists
}

func (r *Registry) RegisterDriver(name, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := DefaultRegistry.externalDrivers[name]; exists {
		logrus.Debugf("Driver %q is already registered, skipping", name)
		return
	}

	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	DefaultRegistry.externalDrivers[name] = &ExternalDriver{
		Name:   name,
		Path:   path,
		logger: log,
	}
}

func (r *Registry) DiscoverDrivers() error {
	limaShareDir, err := usrlocalsharelima.Dir()
	if err != nil {
		return fmt.Errorf("failed to determine Lima share directory: %w", err)
	}
	stdDriverDir := filepath.Join(filepath.Dir(limaShareDir), "libexec", "lima", "drivers")

	if _, err := os.Stat(stdDriverDir); err == nil {
		if err := r.discoverDriversInDir(stdDriverDir); err != nil {
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
		fmt.Printf("Skipping %s: does not start with 'lima-driver-'\n", base)
		return
	}

	name := strings.TrimPrefix(base, "lima-driver-")
	name = strings.TrimSuffix(name, filepath.Ext(name))

	r.RegisterDriver(name, path)
}

func isExecutable(mode os.FileMode) bool {
	return mode&0111 != 0
}

var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry()
	if err := DefaultRegistry.DiscoverDrivers(); err != nil {
		logrus.Warnf("Error discovering drivers: %v", err)
	}
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
