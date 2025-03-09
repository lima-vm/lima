// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"
	"time"

	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
)

const launchHostAgentForeground = false

func Restart(ctx context.Context, inst *store.Instance) error {
	if err := StopGracefully(inst); err != nil {
		return err
	}

	if err := waitForInstanceShutdown(ctx, inst); err != nil {
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
	StopForcibly(inst)

	if err := networks.Reconcile(ctx, inst.Name); err != nil {
		return err
	}

	if err := Start(ctx, inst, "", launchHostAgentForeground); err != nil {
		return err
	}

	return nil
}

func waitForInstanceShutdown(ctx context.Context, inst *store.Instance) error {
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updatedInst, err := store.Inspect(inst.Name)
			if err != nil {
				return errors.New("failed to inspect instance status: " + err.Error())
			}

			if updatedInst.Status == store.StatusStopped {
				return nil
			}
		case <-ctx2.Done():
			return errors.New("timed out waiting for instance to stop")
		}
	}
}
