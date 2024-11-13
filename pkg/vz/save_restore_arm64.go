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

	logrus.Info("Pausing VZ")
	if err := vm.Pause(); err != nil {
		return err
	}

	// If we can't stop the machine after pausing, saving the machine state will be useless.
	// So we should check this before saving the machine state.
	if !vm.CanStop() {
		return fmt.Errorf("can't stop the VZ machine after pausing")
	}

	logrus.Info("Saving VZ machine state for resuming later")
	if err := vm.SaveMachineStateToPath(machineStatePath); err != nil {
		// If we fail to save the machine state, we should resume the machine to call RequestStop() later
		logrus.WithError(err).Errorf("Failed to save the machine state to %q", machineStatePath)
		if resumeError := vm.Resume(); resumeError != nil {
			return resumeError
		}
		return err
	}

	logrus.Info("Stopping VZ")
	if err := vm.Stop(); err != nil {
		// If we fail to stop the machine, we should resume the machine to call RequestStop() later
		logrus.WithError(err).Error("Failed to stop the VZ machine")
		if resumeError := vm.Resume(); resumeError != nil {
			return resumeError
		}
		return err
	}
	return nil
}

func restoreVM(vm *vz.VirtualMachine, machineStatePath string) error {
	if _, err := os.Stat(machineStatePath); err != nil {
		return err
	}
	logrus.Info("Saved VZ machine state found, resuming VZ")
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
