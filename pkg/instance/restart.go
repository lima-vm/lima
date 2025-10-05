// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/autostart"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/networks/reconcile"
)

const (
	launchHostAgentForeground = false
)

func Restart(ctx context.Context, inst *limatype.Instance, showProgress bool) error {
	if err := StopGracefully(ctx, inst, true); err != nil {
		return err
	}

	// Network reconciliation will be performed by the process launched by the autostart manager
	if registered, err := autostart.IsRegistered(ctx, inst); err != nil && !errors.Is(err, autostart.ErrNotSupported) {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	} else if !registered {
		if err := reconcile.Reconcile(ctx, inst.Name); err != nil {
			return err
		}
	}

	if err := Start(ctx, inst, launchHostAgentForeground, showProgress); err != nil {
		return err
	}

	return nil
}

func RestartForcibly(ctx context.Context, inst *limatype.Instance, showProgress bool) error {
	logrus.Info("Restarting the instance forcibly")
	StopForcibly(inst)

	if registered, err := autostart.IsRegistered(ctx, inst); err != nil && !errors.Is(err, autostart.ErrNotSupported) {
		return fmt.Errorf("failed to check if the autostart entry for instance %q is registered: %w", inst.Name, err)
	} else if !registered {
		if err := reconcile.Reconcile(ctx, inst.Name); err != nil {
			return err
		}
	}

	if err := Start(ctx, inst, launchHostAgentForeground, showProgress); err != nil {
		return err
	}

	return nil
}
