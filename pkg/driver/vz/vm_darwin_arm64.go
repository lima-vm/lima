//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/progressbar"
)

func getMacMachineIdentifier(identifier string) (machineIdentifier, error) {
	// Empty VzIdentifier can be created on cloning an instance.
	if st, err := os.Stat(identifier); err != nil || (st != nil && st.Size() == 0) {
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		machineIdentifier, err := vz.NewMacMachineIdentifier()
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(identifier, machineIdentifier.DataRepresentation(), 0o666)
		if err != nil {
			return nil, err
		}
		return machineIdentifier, nil
	}
	return vz.NewMacMachineIdentifierWithDataPath(identifier)
}

func newMacPlatformConfiguration(inst *limatype.Instance) (vz.PlatformConfiguration, error) {
	identifierFile := filepath.Join(inst.Dir, filenames.VzIdentifier)
	machineIdentifier, err := getMacMachineIdentifier(identifierFile)
	if err != nil {
		return nil, err
	}

	var hwModelData []byte
	hwModelFile := filepath.Join(inst.Dir, filenames.VzHwModel)
	if osutil.FileExists(hwModelFile) {
		hwModelData, err = os.ReadFile(hwModelFile)
		if err != nil {
			return nil, err
		}
	} else {
		if err = ensureIPSW(inst.Dir); err != nil {
			return nil, err
		}
		ipsw := filepath.Join(inst.Dir, filenames.ImageIPSW)
		ipswImage, err := vz.LoadMacOSRestoreImageFromPath(ipsw)
		if err != nil {
			return nil, err
		}
		ipswMostFeatureFul := ipswImage.MostFeaturefulSupportedConfiguration()
		hwModelData = ipswMostFeatureFul.HardwareModel().DataRepresentation()
		if err = os.WriteFile(hwModelFile, hwModelData, 0o666); err != nil {
			return nil, err
		}
	}
	hwModel, err := vz.NewMacHardwareModelWithData(hwModelData)
	if err != nil {
		return nil, err
	}

	auxFile := filepath.Join(inst.Dir, filenames.VzAux)
	var auxOps []vz.NewMacAuxiliaryStorageOption
	if !osutil.FileExists(auxFile) {
		auxOps = append(auxOps, vz.WithCreatingMacAuxiliaryStorage(hwModel))
	}
	aux, err := vz.NewMacAuxiliaryStorage(auxFile, auxOps...)
	if err != nil {
		return nil, err
	}

	platformConfig, err := vz.NewMacPlatformConfiguration(
		vz.WithMacMachineIdentifier(machineIdentifier.(*vz.MacMachineIdentifier)),
		vz.WithMacHardwareModel(hwModel),
		vz.WithMacAuxiliaryStorage(aux),
	)
	if err != nil {
		return nil, err
	}
	return platformConfig, nil
}

func newMacGraphicsDeviceConfiguration(x, y, pixelsPerInch int64) (vz.GraphicsDeviceConfiguration, error) {
	graphicsDeviceConfiguration, err := vz.NewMacGraphicsDeviceConfiguration()
	if err != nil {
		return nil, err
	}
	scanoutConfiguration, err := vz.NewMacGraphicsDisplayConfiguration(x, y, pixelsPerInch)
	if err != nil {
		return nil, err
	}
	graphicsDeviceConfiguration.SetDisplays(scanoutConfiguration)
	return graphicsDeviceConfiguration, nil
}

func newMacPointingDeviceConfiguration() (vz.PointingDeviceConfiguration, error) {
	return vz.NewMacTrackpadConfiguration()
}

func newMacKeyboardConfiguration() (vz.KeyboardConfiguration, error) {
	return vz.NewMacKeyboardConfiguration()
}

func newMacOSBootLoader() (vz.BootLoader, error) {
	return vz.NewMacOSBootLoader()
}

func installMacOS(ctx context.Context, vm *vz.VirtualMachine, ipsw string) error {
	installer, err := vz.NewMacOSInstaller(vm, ipsw)
	if err != nil {
		return err
	}

	const barResolution = 100
	bar, err := progressbar.New(barResolution)
	if err != nil {
		return err
	}
	bar.Start()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logrus.WithError(ctx.Err()).Info("cancelling macOS installation")
				return
			case <-installer.Done():
				logrus.WithError(ctx.Err()).Info("macOS installer exited")
				bar.SetCurrent(barResolution)
				return
			case <-ticker.C:
				progress := installer.FractionCompleted() * barResolution
				bar.SetCurrent(int64(progress))
			}
		}
	}()

	err = installer.Install(ctx)
	bar.Finish()
	if err != nil {
		return err
	}

	stopped, err := vm.RequestStop()
	if err != nil {
		return fmt.Errorf("failed to stop VM after installation: %w", err)
	}
	if !stopped {
		logrus.WithError(err).Warn("VM did not stop after installation")
		if err = vm.Stop(); err != nil {
			return fmt.Errorf("failed to force stop VM after installation: %w", err)
		}
	}
	return nil
}
