// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"context"
	"fmt"

	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/store"
)

func Del(ctx context.Context, inst *store.Instance, tag string) error {
	limaDriver, err := driverutil.CreateConfiguredDriver(inst, 0)
	if err != nil {
		return fmt.Errorf("failed to create driver instance: %w", err)
	}

	return limaDriver.DeleteSnapshot(ctx, tag)
}

func Save(ctx context.Context, inst *store.Instance, tag string) error {
	limaDriver, err := driverutil.CreateConfiguredDriver(inst, 0)
	if err != nil {
		return fmt.Errorf("failed to create driver instance: %w", err)
	}
	return limaDriver.CreateSnapshot(ctx, tag)
}

func Load(ctx context.Context, inst *store.Instance, tag string) error {
	limaDriver, err := driverutil.CreateConfiguredDriver(inst, 0)
	if err != nil {
		return fmt.Errorf("failed to create driver instance: %w", err)
	}
	return limaDriver.ApplySnapshot(ctx, tag)
}

func List(ctx context.Context, inst *store.Instance) (string, error) {
	limaDriver, err := driverutil.CreateConfiguredDriver(inst, 0)
	if err != nil {
		return "", fmt.Errorf("failed to create driver instance: %w", err)
	}

	return limaDriver.ListSnapshots(ctx)
}
