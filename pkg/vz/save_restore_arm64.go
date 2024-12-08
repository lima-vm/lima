//go:build darwin && arm64 && !no_vz

package vz

import (
	"errors"
	"fmt"
	"os"

	"github.com/Code-Hex/vz/v3"
	"github.com/sirupsen/logrus"
)

func saveVM(vm *vz.VirtualMachine, machineStatePath string) error {
	if !vm.CanPause() {
		return fmt.Errorf("can't pause the VZ machine")
	}

	// Remove the old machine state file if it exists,
	// because saving the machine state will fail if the file already exists.
	if err := os.Remove(machineStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.WithError(err).Errorf("Failed to remove the old VZ machine state file %q", machineStatePath)
		return err
	}

	logrus.Info("Pausing VZ machine for saving the machine state")
	if err := vm.Pause(); err != nil {
		logrus.WithError(err).Error("Failed to pause the VZ machine")
		return err
	}

	if err := savePausedVM(vm, machineStatePath); err != nil {
		// If we fail to save the machine state, we should resume the machine before returning the error.
		if resumeError := vm.Resume(); resumeError != nil {
			logrus.WithError(resumeError).Error("Failed to resume the VZ machine after pausing")
			return resumeError
		}
		return err
	}

	return nil
}

func savePausedVM(vm *vz.VirtualMachine, machineStatePath string) error {
	// If we can't stop the machine after pausing, saving the machine state will be useless.
	// So we should check this before saving the machine state.
	if !vm.CanStop() {
		return fmt.Errorf("can't stop the VZ machine")
	}

	logrus.Info("Saving VZ machine state for restoring later")
	if err := vm.SaveMachineStateToPath(machineStatePath); err != nil {
		logrus.WithError(err).Errorf("Failed to save the machine state to %q", machineStatePath)
		return err
	}

	logrus.Info("Stopping VZ machine after saving the machine state")
	if err := vm.Stop(); err != nil {
		logrus.WithError(err).Error("Failed to stop the VZ machine")
		return err
	}
	return nil
}

func restoreVM(vm *vz.VirtualMachine, machineStatePath string) error {
	if _, err := os.Stat(machineStatePath); err != nil {
		return err
	}
	logrus.Infof("Resuming VZ machine from %q", machineStatePath)
	if err := vm.RestoreMachineStateFromURL(machineStatePath); err != nil {
		return err
	}
	if err := vm.Resume(); err != nil {
		return err
	}
	// Remove the machine state file after resuming the machine
	if err := os.Remove(machineStatePath); err != nil {
		// We should log the error but continue the process, because the machine state is already restored
		logrus.WithError(err).Errorf("Failed to remove the VZ machine state file %q", machineStatePath)
	}
	return nil
}
