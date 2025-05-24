//go:build !windows || no_wsl

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"errors"
	"net"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/store"
)

var ErrUnsupported = errors.New("vm driver 'wsl2' requires Windows 10 build 19041 or later (Hint: try recompiling Lima if you are seeing this error on Windows 10+)")

const Enabled = false

type LimaWslDriver struct {
	Instance *store.Instance

	SSHLocalPort int
	VSockPort    int
	VirtioPort   string
}

var _ driver.Driver = (*LimaWslDriver)(nil)

func New() *LimaWslDriver {
	return &LimaWslDriver{}
}

func (l *LimaWslDriver) GetVirtioPort() string {
	return l.VirtioPort
}

func (l *LimaWslDriver) GetVSockPort() int {
	return l.VSockPort
}

func (l *LimaWslDriver) Validate() error {
	return ErrUnsupported
}

func (l *LimaWslDriver) Initialize(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) CreateDisk(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) Start(_ context.Context) (chan error, error) {
	return nil, ErrUnsupported
}

func (l *LimaWslDriver) Stop(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) CanRunGUI() bool {
	return false
}

func (l *LimaWslDriver) RunGUI() error {
	return ErrUnsupported
}

func (l *LimaWslDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) GetDisplayConnection(_ context.Context) (string, error) {
	return "", ErrUnsupported
}

func (l *LimaWslDriver) CreateSnapshot(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) ApplySnapshot(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", ErrUnsupported
}

func (l *LimaWslDriver) Register(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) Unregister(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaWslDriver) ForwardGuestAgent() bool {
	return false
}

func (l *LimaWslDriver) GuestAgentConn(_ context.Context) (net.Conn, error) {
	return nil, ErrUnsupported
}

func (l *LimaWslDriver) Name() string {
	return "vz"
}

func (l *LimaWslDriver) SetConfig(inst *store.Instance, sshLocalPort int) {
	l.Instance = inst
	l.SSHLocalPort = sshLocalPort
}
