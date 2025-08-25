// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ext

import (
	"context"
	"fmt"
	"net"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/store"
)

type LimaExtDriver struct {
	Instance *store.Instance
}

var _ driver.Driver = (*LimaExtDriver)(nil)

func New() *LimaExtDriver {
	return &LimaExtDriver{}
}

func (l *LimaExtDriver) Configure(inst *store.Instance) *driver.ConfiguredDriver {
	l.Instance = inst

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaExtDriver) Validate(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) Start(_ context.Context) (chan error, error) {
	errCh := make(chan error)

	return errCh, nil
}

func (l *LimaExtDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "ext", *l.Instance.Config.Video.Display)
}

func (l *LimaExtDriver) Stop(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "", nil
}

func (l *LimaExtDriver) Info() driver.Info {
	var info driver.Info
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}
	info.DriverName = "ext"
	info.CanRunGUI = false
	return info
}

func (l *LimaExtDriver) Initialize(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) CreateDisk(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) Unregister(_ context.Context) error {
	return nil
}

func (l *LimaExtDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaExtDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaExtDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaExtDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaExtDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaExtDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaExtDriver) ForwardGuestAgent() bool {
	// If driver is not providing, use host agent
	return true
}
