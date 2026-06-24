//go:build darwin && arm64
// +build darwin,arm64

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_14_arm64.h"
*/
import "C"
import "github.com/Code-Hex/vz/v3/internal/objc"

// ValidateSaveRestoreSupport Determines whether the framework can save or restore the VM’s current configuration.
//
// Verify that a virtual machine with this configuration is savable.
// Not all configuration options can be safely saved and restored from file.
//
// If this evaluates to false, the caller should expect future calls to `(*VirtualMachine).SaveMachineStateToPath` to fail.
// error If not nil, assigned with an error describing the unsupported configuration option.
func (v *VirtualMachineConfiguration) ValidateSaveRestoreSupport() (bool, error) {
	nserrPtr := newNSErrorAsNil()
	ret := C.validateSaveRestoreSupportWithError(objc.Ptr(v), &nserrPtr)
	err := newNSError(nserrPtr)
	if err != nil {
		return false, err
	}
	return (bool)(ret), nil
}
