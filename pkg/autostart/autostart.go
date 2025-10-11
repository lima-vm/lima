// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package autostart manage start at login unit files for darwin/linux
package autostart

import (
	"context"
	"runtime"
	"sync"

	"github.com/lima-vm/lima/v2/pkg/autostart/systemd"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// IsRegistered checks if the instance is registered to start at login.
func IsRegistered(ctx context.Context, inst *limatype.Instance) (bool, error) {
	return manager().IsRegistered(ctx, inst)
}

// RegisterToStartAtLogin creates a start-at-login entry for the instance.
func RegisterToStartAtLogin(ctx context.Context, inst *limatype.Instance) error {
	return manager().RegisterToStartAtLogin(ctx, inst)
}

// UnregisterFromStartAtLogin deletes the start-at-login entry for the instance.
func UnregisterFromStartAtLogin(ctx context.Context, inst *limatype.Instance) error {
	return manager().UnregisterFromStartAtLogin(ctx, inst)
}

// AutoStartedIdentifier returns the identifier if the current process was started by the autostart manager.
func AutoStartedIdentifier() string {
	return manager().AutoStartedIdentifier()
}

// RequestStart requests to start the instance by identifier.
func RequestStart(ctx context.Context, inst *limatype.Instance) error {
	return manager().RequestStart(ctx, inst)
}

// RequestStop requests to stop the instance by identifier.
func RequestStop(ctx context.Context, inst *limatype.Instance) (bool, error) {
	return manager().RequestStop(ctx, inst)
}

type autoStartManager interface {
	// Registration
	IsRegistered(ctx context.Context, inst *limatype.Instance) (bool, error)
	RegisterToStartAtLogin(ctx context.Context, inst *limatype.Instance) error
	UnregisterFromStartAtLogin(ctx context.Context, inst *limatype.Instance) error

	// Status
	AutoStartedIdentifier() string

	// Operation
	// RequestStart requests to start the instance by identifier.
	RequestStart(ctx context.Context, inst *limatype.Instance) error
	// RequestStop requests to stop the instance by identifier.
	RequestStop(ctx context.Context, inst *limatype.Instance) (bool, error)
}

var manager = sync.OnceValue(func() autoStartManager {
	switch runtime.GOOS {
	case "darwin":
		return Launchd
	case "linux":
		if systemd.IsRunningSystemd() {
			return Systemd
		}
		// TODO: support other init systems
	}
	return &notSupportedManager{}
})
