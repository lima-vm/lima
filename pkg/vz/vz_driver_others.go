//go:build !darwin || no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"net"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/store"
)

var ErrUnsupported = errors.New("vm driver 'vz' needs macOS 13 or later (Hint: try recompiling Lima if you are seeing this error on macOS 13)")

const Enabled = false

type LimaVzDriver struct {
	Instance *store.Instance

	SSHLocalPort int
	VSockPort    int
	VirtioPort   string
}

var _ driver.Driver = (*LimaVzDriver)(nil)

func New() *LimaVzDriver {
	return &LimaVzDriver{}
}

func (l *LimaVzDriver) GetVirtioPort() string {
	return l.VirtioPort
}

func (l *LimaVzDriver) GetVSockPort() int {
	return l.VSockPort
}

func (l *LimaVzDriver) Validate() error {
	return ErrUnsupported
}

func (l *LimaVzDriver) Initialize(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) CreateDisk(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) Start(_ context.Context) (chan error, error) {
	return nil, ErrUnsupported
}

func (l *LimaVzDriver) Stop(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) CanRunGUI() bool {
	return false
}

func (l *LimaVzDriver) RunGUI() error {
	return ErrUnsupported
}

func (l *LimaVzDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) GetDisplayConnection(_ context.Context) (string, error) {
	return "", ErrUnsupported
}

func (l *LimaVzDriver) CreateSnapshot(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) ApplySnapshot(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", ErrUnsupported
}

func (l *LimaVzDriver) Register(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) Unregister(_ context.Context) error {
	return ErrUnsupported
}

func (l *LimaVzDriver) ForwardGuestAgent() bool {
	return false
}

func (l *LimaVzDriver) GuestAgentConn(_ context.Context) (net.Conn, error) {
	return nil, ErrUnsupported
}

func (l *LimaVzDriver) Name() string {
	return "vz"
}

func (l *LimaVzDriver) SetConfig(inst *store.Instance, sshLocalPort int) {
	l.Instance = inst
	l.SSHLocalPort = sshLocalPort
}
