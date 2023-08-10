package stop

import (
	"context"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/driverutil"

	"github.com/lima-vm/lima/pkg/store"
)

func Unregister(ctx context.Context, inst *store.Instance) error {
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	limaDriver := driverutil.CreateTargetDriverInstance(&driver.BaseDriver{
		Instance: inst,
		Yaml:     y,
	})

	return limaDriver.Unregister(ctx)
}
