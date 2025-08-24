// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/store"
)

type mockDriver struct {
	Name string
}

func newMockDriver(name string) *mockDriver {
	return &mockDriver{Name: name}
}

var _ driver.Driver = (*mockDriver)(nil)

func (m *mockDriver) Validate(_ context.Context) error                           { return nil }
func (m *mockDriver) Initialize(_ context.Context) error                         { return nil }
func (m *mockDriver) CreateDisk(_ context.Context) error                         { return nil }
func (m *mockDriver) Start(_ context.Context) (chan error, error)                { return nil, nil }
func (m *mockDriver) Stop(_ context.Context) error                               { return nil }
func (m *mockDriver) RunGUI(_ context.Context) error                             { return nil }
func (m *mockDriver) ChangeDisplayPassword(_ context.Context, _ string) error    { return nil }
func (m *mockDriver) DisplayConnection(_ context.Context) (string, error)        { return "", nil }
func (m *mockDriver) CreateSnapshot(_ context.Context, _ string) error           { return nil }
func (m *mockDriver) ApplySnapshot(_ context.Context, _ string) error            { return nil }
func (m *mockDriver) DeleteSnapshot(_ context.Context, _ string) error           { return nil }
func (m *mockDriver) ListSnapshots(_ context.Context) (string, error)            { return "", nil }
func (m *mockDriver) Register(_ context.Context) error                           { return nil }
func (m *mockDriver) Unregister(_ context.Context) error                         { return nil }
func (m *mockDriver) ForwardGuestAgent(_ context.Context) bool                   { return false }
func (m *mockDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) { return nil, "", nil }
func (m *mockDriver) Info(_ context.Context) driver.Info                         { return driver.Info{DriverName: m.Name} }
func (m *mockDriver) Configure(_ context.Context, _ *store.Instance) *driver.ConfiguredDriver {
	return nil
}

func TestRegister(t *testing.T) {
	BackupRegistry(t)

	ctx := t.Context()
	mockDrv := newMockDriver("test-driver")
	mockDrv2 := newMockDriver("test-driver-2")
	Register(ctx, mockDrv)
	Register(ctx, mockDrv2)

	assert.Equal(t, len(internalDrivers), 2)
	assert.Equal(t, internalDrivers["test-driver"], mockDrv)
	assert.Equal(t, internalDrivers["test-driver-2"], mockDrv2)

	// Test registering duplicate driver (should not overwrite)
	mockDrv3 := newMockDriver("test-driver")
	Register(ctx, mockDrv3)

	assert.Equal(t, len(internalDrivers), 2)
	assert.Equal(t, internalDrivers["test-driver"], mockDrv)

	driverType := CheckInternalOrExternal("test-driver")
	assert.Equal(t, driverType, Internal)

	extDriver, intDriver, exists := Get("test-driver")
	assert.Equal(t, exists, true)
	assert.Assert(t, extDriver == nil)
	assert.Assert(t, intDriver != nil)
	assert.Equal(t, intDriver.Info(ctx).DriverName, "test-driver")

	vmTypes := List()
	assert.Equal(t, vmTypes["test-driver-2"], Internal)
}

func TestDiscoverDriversInDir(t *testing.T) {
	BackupRegistry(t)

	tempDir := t.TempDir()

	var driverPath string
	driverName := "mockext"
	if runtime.GOOS == "windows" {
		driverPath = filepath.Join(tempDir, "lima-driver-"+driverName+".exe")
	} else {
		driverPath = filepath.Join(tempDir, "lima-driver-"+driverName)
	}

	err := os.WriteFile(driverPath, []byte(""), 0o755)
	assert.NilError(t, err)

	err = discoverDriversInDir(tempDir)
	assert.NilError(t, err)

	assert.Equal(t, len(ExternalDrivers), 1)
	extDriver := ExternalDrivers[driverName]
	assert.Assert(t, extDriver != nil)
	assert.Equal(t, extDriver.Name, driverName)
	assert.Equal(t, extDriver.Path, driverPath)

	driverType := CheckInternalOrExternal(driverName)
	assert.Equal(t, driverType, External)

	extDriver, intDriver, exists := Get(driverName)
	assert.Equal(t, exists, true)
	assert.Assert(t, extDriver != nil)
	assert.Assert(t, intDriver == nil)
	assert.Equal(t, extDriver.Name, driverName)

	vmTypes := List()
	assert.Equal(t, vmTypes[driverName], driverPath)
}

func TestRegisterDriverFile(t *testing.T) {
	BackupRegistry(t)

	tests := []struct {
		name         string
		filename     string
		expectDriver bool
		expectedName string
	}{
		{
			name:         "valid driver file",
			filename:     "lima-driver-test",
			expectDriver: runtime.GOOS != "windows",
			expectedName: "test",
		},
		{
			name:         "valid driver file with extension on Windows",
			filename:     "lima-driver-windows.exe",
			expectDriver: runtime.GOOS == "windows",
			expectedName: "windows",
		},
		{
			name:         "invalid filename - no prefix",
			filename:     "not-a-driver",
			expectDriver: false,
		},
		{
			name:         "invalid filename - wrong prefix",
			filename:     "driver-lima-test",
			expectDriver: false,
		},
		{
			name:         "empty name after prefix",
			filename:     "lima-driver-",
			expectDriver: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ExternalDrivers = make(map[string]*ExternalDriver)
			registerDriverFile(filepath.Join("/test/path", tt.filename))

			if tt.expectDriver {
				assert.Equal(t, len(ExternalDrivers), 1)
				extDriver := ExternalDrivers[tt.expectedName]
				assert.Assert(t, extDriver != nil)
				assert.Equal(t, extDriver.Name, tt.expectedName)
				assert.Equal(t, extDriver.Path, filepath.Join("/test/path", tt.filename))
			} else {
				assert.Equal(t, len(ExternalDrivers), 0)
			}
		})
	}
}

func TestGet(t *testing.T) {
	BackupRegistry(t)

	mockDrv := newMockDriver("internal-test")
	Register(t.Context(), mockDrv)

	extDriver, intDriver, exists := Get("internal-test")
	assert.Equal(t, exists, true)
	assert.Assert(t, extDriver == nil)
	assert.Equal(t, intDriver, mockDrv)

	registerExternalDriver("external-test", "/path/to/external")

	extDriver, intDriver, exists = Get("external-test")
	assert.Equal(t, exists, true)
	assert.Assert(t, extDriver != nil)
	assert.Assert(t, intDriver == nil)
	assert.Equal(t, extDriver.Name, "external-test")

	extDriver, intDriver, exists = Get("non-existent")
	assert.Equal(t, exists, false)
	assert.Assert(t, extDriver == nil)
	assert.Assert(t, intDriver == nil)
}

func TestList(t *testing.T) {
	BackupRegistry(t)

	vmTypes := List()
	assert.Equal(t, len(vmTypes), 0)

	mockDrv := newMockDriver("internal-test")
	Register(t.Context(), mockDrv)

	vmTypes = List()
	assert.Equal(t, len(vmTypes), 1)
	assert.Equal(t, vmTypes["internal-test"], Internal)

	registerExternalDriver("external-test", "/path/to/external")

	vmTypes = List()
	assert.Equal(t, len(vmTypes), 2)
	assert.Equal(t, vmTypes["internal-test"], Internal)
	assert.Equal(t, vmTypes["external-test"], "/path/to/external")
}

func BackupRegistry(t *testing.T) {
	originalExternalDrivers := ExternalDrivers
	originalInternalDrivers := internalDrivers
	t.Cleanup(func() {
		ExternalDrivers = originalExternalDrivers
		internalDrivers = originalInternalDrivers
	})

	internalDrivers = make(map[string]driver.Driver)
	ExternalDrivers = make(map[string]*ExternalDriver)
}
