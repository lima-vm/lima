package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_11.h"
*/
import "C"
import (
	"github.com/Code-Hex/vz/v3/internal/objc"
)

// VirtioEntropyDeviceConfiguration is used to expose a source of entropy for the guest operating system’s random-number generator.
// When you create this object and add it to your virtual machine’s configuration, the virtual machine configures a Virtio-compliant
// entropy device. The guest operating system uses this device as a seed to generate random numbers.
//
// see: https://developer.apple.com/documentation/virtualization/vzvirtioentropydeviceconfiguration?language=objc
type VirtioEntropyDeviceConfiguration struct {
	*pointer
}

// NewVirtioEntropyDeviceConfiguration creates a new Virtio Entropy Device confiuration.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewVirtioEntropyDeviceConfiguration() (*VirtioEntropyDeviceConfiguration, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	config := &VirtioEntropyDeviceConfiguration{
		pointer: objc.NewPointer(
			C.newVZVirtioEntropyDeviceConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *VirtioEntropyDeviceConfiguration) {
		objc.Release(self)
	})
	return config, nil
}
