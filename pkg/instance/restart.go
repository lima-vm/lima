// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"

	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/sirupsen/logrus"
)

const launchHostAgentForeground = false

func Restart(ctx context.Context, inst *store.Instance) error {
	if err := StopGracefully(ctx, inst, true); err != nil {
		return err
	}

	if err := networks.Reconcile(ctx, inst.Name); err != nil {
		return err
	}

	if err := Start(ctx, inst, "", launchHostAgentForeground); err != nil {
		return err
	}

	return nil
}

func RestartForcibly(ctx context.Context, inst *store.Instance) error {
	logrus.Info("Restarting the instance forcibly")
	StopForcibly(inst)

	if err := networks.Reconcile(ctx, inst.Name); err != nil {
		return err
	}

	if err := Start(ctx, inst, "", launchHostAgentForeground); err != nil {
		return err
	}

	return nil
}
