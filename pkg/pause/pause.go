package pause

import (
	"context"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/qemu"
	"github.com/lima-vm/lima/pkg/store"
)

func stop(ctx context.Context, instName, instDir string, y *limayaml.LimaYAML) error {
	qCfg := qemu.Config{
		Name:        instName,
		InstanceDir: instDir,
		LimaYAML:    y,
	}
	return qemu.Stop(qCfg)
}

func Suspend(ctx context.Context, inst *store.Instance) error {
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	if err := stop(ctx, inst.Name, inst.Dir, y); err != nil {
		return err
	}

	inst.Status = store.StatusPaused
	return nil
}

func cont(ctx context.Context, instName, instDir string, y *limayaml.LimaYAML) error {
	qCfg := qemu.Config{
		Name:        instName,
		InstanceDir: instDir,
		LimaYAML:    y,
	}
	return qemu.Cont(qCfg)
}

func Resume(ctx context.Context, inst *store.Instance) error {
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}

	if err := cont(ctx, inst.Name, inst.Dir, y); err != nil {
		return err
	}

	inst.Status = store.StatusRunning
	return nil
}
