package instance

import (
	"context"
	"fmt"
	"os"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/store"
)

func Delete(ctx context.Context, inst *store.Instance, force bool) error {
	if inst.Protected {
		return fmt.Errorf("instance is protected to prohibit accidental removal (Hint: use `limactl unprotect`)")
	}
	if !force && inst.Status != store.StatusStopped {
		return fmt.Errorf("expected status %q, got %q", store.StatusStopped, inst.Status)
	}

	StopForcibly(inst)

	if err := unregister(ctx, inst); err != nil {
		return fmt.Errorf("failed to unregister %q: %w", inst.Dir, err)
	}

	if err := os.RemoveAll(inst.Dir); err != nil {
		return fmt.Errorf("failed to remove %q: %w", inst.Dir, err)
	}

	return nil
}

func unregister(ctx context.Context, inst *store.Instance) error {
	instConfig, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance:   inst,
		InstConfig: instConfig,
	})

	return limaDriver.Unregister(ctx)
}
