//go:build darwin && arm64
// +build darwin,arm64

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_13_arm64.h"
*/
import "C"
import (
	"github.com/Code-Hex/vz/v3/internal/objc"
)

// MacTrackpadConfiguration is a struct that defines the configuration
// for a Mac trackpad.
//
// This device is only recognized by virtual machines running macOS 13.0 and later.
// In order to support both macOS 13.0 and earlier guests, VirtualMachineConfiguration.pointingDevices
// can be set to an array containing both a MacTrackpadConfiguration and
// a USBScreenCoordinatePointingDeviceConfiguration object. macOS 13.0 and later guests will use
// the multi-touch trackpad device, while earlier versions of macOS will use the USB pointing device.
//
// see: https://developer.apple.com/documentation/virtualization/vzmactrackpadconfiguration?language=objc
type MacTrackpadConfiguration struct {
	*pointer

	*basePointingDeviceConfiguration
}

var _ PointingDeviceConfiguration = (*MacTrackpadConfiguration)(nil)

// NewMacTrackpadConfiguration creates a new MacTrackpadConfiguration.
//
// This is only supported on macOS 13 and newer, error will
// be returned on older versions.
func NewMacTrackpadConfiguration() (*MacTrackpadConfiguration, error) {
	if err := macOSAvailable(13); err != nil {
		return nil, err
	}
	config := &MacTrackpadConfiguration{
		pointer: objc.NewPointer(
			C.newVZMacTrackpadConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *MacTrackpadConfiguration) {
		objc.Release(self)
	})
	return config, nil
}
