// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	networks "github.com/lima-vm/lima/v2/pkg/networks/reconcile"
)

const (
	launchHostAgentForeground = false
)

func Restart(ctx context.Context, inst *limatype.Instance, showProgress bool) error {
	if err := StopGracefully(ctx, inst, true); err != nil {
		return err
	}

	if err := networks.Reconcile(ctx, inst.Name); err != nil {
		return err
	}

	if err := Start(ctx, inst, launchHostAgentForeground, showProgress); err != nil {
		return err
	}

	return nil
}

func RestartForcibly(ctx context.Context, inst *limatype.Instance, showProgress bool) error {
	logrus.Info("Restarting the instance forcibly")
	StopForcibly(inst)

	if err := networks.Reconcile(ctx, inst.Name); err != nil {
		return err
	}

	if err := Start(ctx, inst, launchHostAgentForeground, showProgress); err != nil {
		return err
	}

	return nil
}
