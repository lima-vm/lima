package pause

import (
	"context"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/store"
)

func Suspend(ctx context.Context, inst *store.Instance) error {
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
		Yaml:     y,
	})

	if err := limaDriver.Suspend(ctx); err != nil {
		return err
	}

	inst.Status = store.StatusPaused
	return nil
}

func Resume(ctx context.Context, inst *store.Instance) error {
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
		Yaml:     y,
	})

	if err := limaDriver.Resume(ctx); err != nil {
		return err
	}

	inst.Status = store.StatusRunning
	return nil
}
