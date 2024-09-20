package snapshot

import (
	"context"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/store"
)

func Del(ctx context.Context, inst *store.Instance, tag string) error {
	instConfig, err := inst.LoadYAML()
	if err != nil {
		return err
	}
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance:   inst,
		InstConfig: instConfig,
	})
	return limaDriver.DeleteSnapshot(ctx, tag)
}

func Save(ctx context.Context, inst *store.Instance, tag string) error {
	instConfig, err := inst.LoadYAML()
	if err != nil {
		return err
	}
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance:   inst,
		InstConfig: instConfig,
	})
	return limaDriver.CreateSnapshot(ctx, tag)
}

func Load(ctx context.Context, inst *store.Instance, tag string) error {
	instConfig, err := inst.LoadYAML()
	if err != nil {
		return err
	}
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance:   inst,
		InstConfig: instConfig,
	})
	return limaDriver.ApplySnapshot(ctx, tag)
}

func List(ctx context.Context, inst *store.Instance) (string, error) {
	y, err := inst.LoadYAML()
	if err != nil {
		return "", err
	}
	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance:   inst,
		InstConfig: y,
	})
	return limaDriver.ListSnapshots(ctx)
}
