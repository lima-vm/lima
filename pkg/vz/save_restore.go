//go:build darwin && !arm64 && !no_vz

package vz

import (
	"fmt"
	"runtime"

	"github.com/Code-Hex/vz/v3"
)

func saveVM(vm *vz.VirtualMachine, machineStatePath string) error {
	return fmt.Errorf("save is not supported on the vz driver for the architecture %s", runtime.GOARCH)
}

func restoreVM(vm *vz.VirtualMachine, machineStatePath string) error {
	return fmt.Errorf("restore is not supported on the vz driver for the architecture %s", runtime.GOARCH)
}
